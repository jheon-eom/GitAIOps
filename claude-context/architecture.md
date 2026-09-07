# Notiflex 아키텍처 스냅샷

**작성 시점**: 2026-09-07 (ch8.3)
**책 진도**: ch8 완료 (Kafka·Tempo·CronJob — 관측 3축 + 이벤트 드리븐 + 스케줄러)
**구성**: 컴포넌트 → 연결 → 설정 순으로 읽으면 시스템을 한번에 파악 가능

## 3층 지식 구조

| 문서 | 성격 | 시제 | 갱신 시점 | 로드 방식 |
|------|------|------|----------|----------|
| `CLAUDE.md` | 프로젝트 메타·행동 규칙 | 상시 | 규칙 변경 시 | 매 대화 자동 로드 |
| `claude-context/architecture.md` | **현재** 아키텍처 스냅샷 | **현재** | 큰 구조 변화 후 | AI가 필요 시 참조 |
| `docs/architecture-decisions.md` | ADR 결정 누적 | **과거** | 결정 시점마다 | AI/사람 검토용 |
| `JOURNEY.md` | 시계열 진행·트러블슈팅 | 시계열 | 각 서브챕터 완료 시 | AI가 필요 시 참조 |

- **왜 이 결정을 내렸는가** → ADR
- **지금 어떻게 동작하는가** → 이 파일
- **어떤 문제를 겪었고 어떻게 풀었는가** → JOURNEY

---

## 1. 컴포넌트

### 1.1 클러스터·인프라

| 항목 | 값 |
|------|-----|
| 클러스터 | `notiflex-cluster` (GKE Standard, Zonal) |
| 리전/존 | `asia-northeast3` / `asia-northeast3-a` |
| 노드풀 | `default-pool` e2-medium ×2 · `api-pool` e2-medium ×1 · `worker-pool` e2-standard-2 ×1 · `ops-pool` e2-small ×1 (모두 **Spot**, ch7.2 확장) |
| Workload Identity | `git-ai-ops-practice.svc.id.goog` (ch6.2 활성화, 모든 노드풀에 `--workload-metadata=GKE_METADATA`) |
| Secret Manager CSI | GKE addon, driver=`secrets-store-gke.csi.k8s.io` |
| Gateway API | GKE managed `gke-l7-regional-external-managed` |
| Artifact Registry | `asia-northeast3-docker.pkg.dev/git-ai-ops-practice/notiflex` |
| kubectl context | `gke-sysnet4admin_book_gitaiops` |

### 1.2 애플리케이션 (notiflex namespace — SMB 테넌트)

모든 Rollout Pod은 `nodeSelector: cloud.google.com/gke-nodepool=api-pool`로 api-pool에 고정 (ch7.2).

| 컴포넌트 | 종류 | 관리 | 상세 |
|---------|------|------|------|
| `notiflex-api` | Rollout (Canary) | ArgoCD | image `api:sha-0158fed` (v0.8.0, Kafka Producer+Consumer + OTel 계측), replicas=1, SA=`notiflex-api`, nodeSelector=api-pool |
| `valkey-primary` | StatefulSet | Helm | chart `bitnami/valkey-6.2.19` (app 9.1.2), standalone, replicas=1 (default-pool) |
| `notiflex-secrets` | SecretProviderClass | ArgoCD | provider=`gke`, ref=`projects/.../secrets/valkey-password/versions/latest` |
| `notiflex-api` (Service) | ClusterIP | ArgoCD | stable Service — Canary의 stableService, port 80→8080 |
| `notiflex-api-preview` | ClusterIP | ArgoCD | canary Service — Canary의 canaryService |
| `notiflex-gateway` | Gateway v1 | ArgoCD | classes=`gke-l7-regional-external-managed`, HTTP:80, IP=35.216.118.49 |
| `notiflex-route` | HTTPRoute | ArgoCD | `/` prefix → `notiflex-api:80` (weight=1) |
| `notiflex-healthcheck` (HCP) | HealthCheckPolicy | ArgoCD | HTTP :8080 `/health`, interval 15s, threshold 2/1, timeout 5s |
| `notiflex-healthcheck` (CronJob) | CronJob | ArgoCD | ch8.3 신규, `*/5 * * * *`, ops-pool, curl `/health`→200, OnFailure + backoffLimit 2, history 3/3, ttl 1h |
| `notiflex-api` (SA) | ServiceAccount | ArgoCD | WI annotation → GCP SA `notiflex-secret-reader` |

### 1.2b 애플리케이션 (enterprise namespace — Enterprise 테넌트, ch7.4 신규)

| 컴포넌트 | 종류 | 관리 | 상세 |
|---------|------|------|------|
| `notiflex-api` | Rollout (Canary) | ArgoCD (App=`notiflex-enterprise`) | 동일 이미지 `api:sha-0158fed` (v0.8.0), replicas=1, nodeSelector=api-pool |
| `notiflex-api`, `notiflex-api-preview` | ClusterIP | ArgoCD | SMB와 이름 동일, ns 격리로 충돌 없음. Gateway/HTTPRoute 없음 (내부 전용) |
| `notiflex-api` (SA) | ServiceAccount | ArgoCD | 같은 GSA `notiflex-secret-reader`에 추가 WI 바인딩 |
| `notiflex-secrets` | SecretProviderClass | ArgoCD | notiflex ns와 동일 GSM secret 참조 |
| `enterprise-quota` | ResourceQuota | ArgoCD | pods 3, requests.cpu 100m, requests.memory 128Mi, limits.cpu 500m, limits.memory 256Mi |

Valkey는 notiflex ns의 인스턴스를 `valkey-primary.notiflex.svc.cluster.local:6379` cross-namespace DNS로 공유.

### 1.2c 메시징 (kafka namespace — ch8.1 신규)

Strimzi Operator + Kafka는 모두 worker-pool(e2-standard-2)에 `nodeAffinity`로 고정.

| 컴포넌트 | 종류 | 관리 | 상세 |
|---------|------|------|------|
| `strimzi-cluster-operator` | Deployment | Helm | chart `strimzi/strimzi-kafka-operator 1.2.0`, nodeSelector=worker-pool. `Kafka`/`KafkaNodePool`/`KafkaTopic` CRD 제공 |
| `notiflex-kafka` | Kafka (`kafka.strimzi.io/v1`) | ArgoCD (`notiflex-kafka` App) | KRaft 모드(ZooKeeper 없음), 4.3.0/metadataVersion 4.3-IV0, plain listener 9092, `min.insync.replicas=1` |
| `broker` | KafkaNodePool | ArgoCD | roles=[controller, broker], replicas=1, JBOD 10Gi PVC, `-Xms/Xmx=512m`, requests cpu 300m/1Gi |
| `notifications` | KafkaTopic | ArgoCD | partitions=3, replicas=1 |
| `notiflex-kafka-entity-operator` | Deployment | Strimzi | topicOperator만 활성(KafkaUser 미사용 → userOperator 비활성) |
| Bootstrap Service | ClusterIP | Strimzi 자동 | `notiflex-kafka-kafka-bootstrap.kafka.svc.cluster.local:9092` — 앱이 `KAFKA_BROKER` env로 참조 |

### 1.3 배포·GitOps (argocd, argo-rollouts namespaces)

| 컴포넌트 | 종류 | 관리 | 상세 |
|---------|------|------|------|
| ArgoCD (7종) | Deployment/STS | 수동 설치 | v3.5.2, App of Apps 구조 — `root-app`(수동 부트스트랩)이 `argocd/apps/` 감시 → `notiflex-smb`, `notiflex-enterprise` 자동 관리 (ch7.3) |
| `argo-rollouts` | Deployment | 수동 설치 | v1.10.0, `--server-side` apply로 CRD 등록 |
| GitHub Actions | 외부 CI | 저장소 | `app/**` 변경 시 build+push, 후속 job에서 rollout.yaml image line 자동 커밋 |

### 1.4 관측 가능성 (monitoring namespace)

| 컴포넌트 | 상태 | 상세 |
|---------|------|------|
| Prometheus | Running | kube-prometheus-stack 89.2.0, retention 7d, scrape 30s, CPU 5m (축소) |
| Grafana | Running | v13.2.1, memory limit 512Mi, sidecar로 dashboard/datasource 자동 로드 |
| Alertmanager | Running | 3 PrometheusRule 등록, receiver=null (Slack 미연결) |
| kube-state-metrics | Running | CPU 10m |
| node-exporter | Running (DaemonSet) | 노드당 10m |
| **Loki** | **임시 제거** | ch6.2 CSI CPU 확보용, 향후 ops-pool로 복원 예정 |
| **Fluent Bit** | **임시 제거** | ch6.2 CSI CPU 확보용, 향후 ops-pool로 복원 예정 |
| Tempo | Running (ops-pool) | grafana/tempo 2.9.0 (monolithic, deprecated 경고 무시), OTLP gRPC :4317 + HTTP :4318, persistence 비활성, requests 25m/128Mi. Grafana 데이터소스 `Tempo` 자동 등록(`tracesToLogsV2`+`serviceMap`) |

### 1.5 GCP 리소스 (프로젝트 `git-ai-ops-practice`)

| 리소스 | 이름/식별자 | 목적 |
|--------|-------------|------|
| Secret | `valkey-password` (v1) | Valkey 인증 원본 |
| GCP ServiceAccount | `notiflex-secret-reader@...iam` | K8s Pod가 사용할 신원 |
| IAM (Secret Manager) | `roles/secretmanager.secretAccessor` | 위 GSA에게 부여 |
| IAM (Workload Identity) | `roles/iam.workloadIdentityUser` | K8s SA `notiflex/notiflex-api` ↔ 위 GSA 매핑 |
| Artifact Registry | `notiflex/api` | Docker 이미지 저장소 |

---

## 2. 연결 (누가 누구를 어떻게 부르는가)

### 2.1 트래픽 흐름

```
[인터넷 사용자]
      │  HTTP :80
      ▼
┌─────────────────────────────────┐
│ Gateway: notiflex-gateway        │  IP=35.216.118.49
│  class=gke-l7-regional-external  │  (GKE L7 GCLB)
└─────────────┬───────────────────┘
              │
              ▼
┌─────────────────────────────────┐
│ HTTPRoute: notiflex-route        │  match: PathPrefix "/"
└─────────────┬───────────────────┘  backendRef: notiflex-api:80 (weight=1)
              │
              ▼
┌─────────────────────────────────┐
│ Service: notiflex-api (stable)   │
└─────────────┬───────────────────┘
              │  selector matches Rollout stable ReplicaSet
              ▼
┌─────────────────────────────────┐
│ Rollout Pod (v0.6.0)             │  container port 8080
│  /health /version /id /ping      │  HealthCheckPolicy → :8080/health
└─────────────────────────────────┘
```

**참고**: HTTPRoute의 backendRef는 `notiflex-api`(stable) 하나뿐. Canary weight 진행 시 canary Pod은 별도 preview Service로만 접근 가능(플러그인 없는 Basic Canary 모드).

### 2.1b 헬스체크(스케줄러) 흐름 — ch8.3

```
CronJob: notiflex-healthcheck  (schedule: */5 * * * *, ops-pool)
   │  concurrencyPolicy=Forbid
   ▼
Job → Pod (curlimages/curl:8.10.1, restartPolicy=OnFailure)
   │  args: curl -sS --max-time 5 http://notiflex-api.notiflex.svc.cluster.local/health
   ▼
Service: notiflex-api.notiflex.svc.cluster.local  (ClusterIP :80 → :8080)
   ▼
Rollout stable ReplicaSet Pod (/health → 200 {"status":"ok"})
   │
   ├─ 200 → exit 0 → Job Complete
   └─ else → stderr + exit 1 → OnFailure 재시작 (backoffLimit=2)
```

**관측 연동**: 실패 Job은 kube-state-metrics의 `kube_job_status_failed{job_name=~"notiflex-healthcheck-.*"}`로 노출됨. PrometheusRule을 추가하면 4.4의 Alertmanager로 연결 가능.

### 2.2 상태(카운터) 흐름

```
Rollout Pod
   │
   │  1. loadValkeyPassword()
   │     - 파일 우선: /mnt/secrets/valkey-password
   │     - 없으면: VALKEY_PASSWORD env
   │
   │  2. valkey.NewClient(InitAddress=VALKEY_ADDR, Password=...)
   │     - 10회×3s 재시도 (DNS/Valkey 기동 순서 방어)
   │
   │  3. GET /id 요청 시 → INCR notiflex:id
   ▼
Service: valkey-primary.notiflex.svc.cluster.local:6379
   │
   ▼
StatefulSet: valkey-primary-0 (bitnami/valkey 9.1.2, standalone)
   │
   ▼  AOF (appendonly.aof) → PVC (8Gi, filesystem)
```

**영속성**: Pod 재시작 후에도 카운터 유지 (6.1 검증: 10→11 자연스럽게 이어짐).

### 2.2b 비동기 이벤트 흐름 — ch8.1

```
GET /id 요청
   │
   ▼
Rollout Pod: idHandler
   ├─ 1. Valkey INCR notiflex:id → next
   │
   └─ 2. sarama.SyncProducer.SendMessage(Topic=notifications,
                                          Key=next,
                                          Value=`{"id":next,"pod":<hostname>}`,
                                          Headers=[traceparent, tracestate])   ← OTel 컨텍스트 주입
             │  RequiredAcks=WaitForLocal, Retry.Max=5, Version=V4_3_0_0
             ▼
      Service: notiflex-kafka-kafka-bootstrap.kafka.svc.cluster.local:9092
             │
             ▼
      Kafka Broker (notiflex-kafka-broker-0, worker-pool)
             │  → Topic `notifications` (partitions=3, RF=1) 에 append
             │
             ▼
      같은 프로세스 내 백그라운드 goroutine
      sarama.ConsumerGroup(groupID=notiflex-api, Initial=OffsetNewest)
             │  span=kafka.consume, parent=kafka.produce (헤더에서 컨텍스트 복원)
             ▼
      로그 출력 후 sess.MarkMessage(msg, "")
```

**핵심**: Producer와 Consumer가 같은 Pod의 다른 goroutine이지만, Kafka RecordHeader의 `traceparent` 전파로 하나의 Trace로 묶여 관측된다.

### 2.2c 트레이싱 흐름 (관측 3축 통합) — ch8.2

```
Rollout Pod (OTel Go SDK, ServiceName=notiflex-api, ServiceVersion=v0.8.0)
   │  otelhttp 서버 미들웨어가 GET /id 요청마다 root SPAN_KIND_SERVER 생성
   │  BatchSpanProcessor + AlwaysSample
   │
   ├─ tracer.Start("valkey.incr")                       ─┐
   ├─ tracer.Start("kafka.produce")  (헤더 주입)         │  같은 TraceID
   │     └── Consumer goroutine: tracer.Start(           │
   │           "kafka.consume", kind=CONSUMER,          │
   │            parent=kafka.produce 헤더에서 추출)  ─┘
   │
   │  OTLP gRPC over insecure  →  tempo.monitoring.svc.cluster.local:4317
   ▼
Tempo (monolithic, ops-pool)
   │  ingester → block-builder → local WAL (persistence 비활성, 학습 목적)
   ▼
Grafana ← 데이터소스 `Tempo` (uid=tempo)
   │  tracesToLogsV2 → Loki (spanStart/End ±30s, filterByTraceID)
   └  serviceMap    → Prometheus (uid=prometheus)
```

**실측**: `/id` 한 번 → 4-span waterfall (SERVER → INTERNAL × 2 → CONSUMER), duration 5~30ms.

### 2.3 시크릿 흐름 (Workload Identity 경유)

```
GCP Secret Manager
  └── valkey-password (v1)
        │  roles/secretmanager.secretAccessor 부여됨
        ▼
GCP ServiceAccount: notiflex-secret-reader@git-ai-ops-practice.iam
        │  roles/iam.workloadIdentityUser 로 아래 K8s SA에게 impersonate 허용
        ▼
K8s ServiceAccount: notiflex/notiflex-api
  annotation: iam.gke.io/gcp-service-account=notiflex-secret-reader@...
        │
        │  Pod spec.serviceAccountName=notiflex-api
        ▼
Pod (gke-metadata-server가 OIDC 토큰 발급)
        │
        │  CSI driver=secrets-store-gke.csi.k8s.io 가 SPC 참조
        ▼
SecretProviderClass: notiflex-secrets
  provider: gke
  parameters.secrets:
    - resourceName: projects/git-ai-ops-practice/secrets/valkey-password/versions/latest
      path: valkey-password
        │
        ▼
Pod 파일시스템: /mnt/secrets/valkey-password  (readOnly)
```

**SA 키 파일 없이** OIDC 토큰만으로 GCP Secret Manager 접근. K8s Secret에 비밀번호가 저장되지 않음.

### 2.4 배포·GitOps 흐름

```
개발자 → git push (app/**)
              │
              ▼
GitHub Actions: build-and-push
  1. docker build app/  →  api:sha-XXXXXXX
  2. Artifact Registry push
              │
              ▼
GitHub Actions: update-manifest (같은 워크플로우 후속 job)
  1. sed로 k8s/smb/rollout.yaml image line 갱신
  2. github-actions[bot] 자동 커밋 "[skip ci]"
  3. git push origin main
              │
              ▼
ArgoCD Application: notiflex-smb
  auto-sync: prune=true, selfHeal=true
  ├── Namespace, Service×2, Rollout, Gateway, HTTPRoute, HealthCheckPolicy,
  │   ServiceAccount, SecretProviderClass 모두 sync
  └── Rollout 변경 감지
              │
              ▼
Argo Rollouts controller
  strategy=Canary: steps 20%→pause 30s→50%→pause 30s→80%→pause 30s→100%
  단계별 stable/canary ReplicaSet 조정
  최종 승격 시 stable Service selector가 새 ReplicaSet으로 이동
```

**관리 경계**
- **ArgoCD 소유**:
  - `k8s/smb/` (10개, notiflex ns — ch8.3 CronJob 추가) + `k8s/enterprise/` (6개, enterprise ns)
  - `k8s/kafka/` (3개, kafka ns — KafkaNodePool + Kafka + KafkaTopic, wave=1)
  - `k8s/monitoring/` (dashboard/datasource ConfigMap — Loki, Tempo, notiflex-dashboard, pod-restart-alert)
  - 루트는 `root-app`이 `argocd/apps/` 재귀 감시
- **Helm 소유** (ArgoCD 밖): `valkey` (notiflex), `kube-prometheus` (monitoring), `strimzi` (kafka, ch8.1), `tempo` (monitoring, ch8.2)
- **GKE 시스템**: CSI Secrets Store DaemonSet, gke-metadata-server, kube-dns, GKE Managed Prometheus (gmp-system)

**향후**: 남은 helm 릴리스(kube-prometheus, valkey, strimzi, tempo)를 wave=1 플랫폼 App으로 ArgoCD에 편입 예정. 새 앱 추가 = `argocd/apps/`에 YAML 하나 커밋.

---

## 3. 설정 (수치·값)

### 3.1 Rollout `notiflex-api` (실측)

| 필드 | 값 |
|------|-----|
| replicas | 1 (ch6.1 축소, ch7.2 복원 예정) |
| strategy | canary |
| stableService | notiflex-api |
| canaryService | notiflex-api-preview |
| steps | `[setWeight:20, pause:30s, setWeight:50, pause:30s, setWeight:80, pause:30s]` |
| container image | `.../api:sha-0158fed` (v0.8.0) |
| readinessProbe | HTTP :8080 `/health`, initialDelay 2s, period 5s |
| livenessProbe | HTTP :8080 `/health`, initialDelay 5s, period 10s |
| resources.requests | cpu 10m, memory 32Mi |
| resources.limits | cpu 200m, memory 128Mi |
| serviceAccountName | notiflex-api |
| env | `VALKEY_ADDR=valkey-primary.notiflex.svc.cluster.local:6379`, `VALKEY_PASSWORD_FILE=/mnt/secrets/valkey-password`, `KAFKA_BROKER=notiflex-kafka-kafka-bootstrap.kafka.svc.cluster.local:9092`, `OTEL_EXPORTER_OTLP_ENDPOINT=tempo.monitoring.svc.cluster.local:4317` |
| volumes.secrets | CSI driver=`secrets-store-gke.csi.k8s.io`, SPC=`notiflex-secrets` |

### 3.2 Gateway·HTTPRoute·HealthCheckPolicy

| 리소스 | 설정 |
|--------|------|
| Gateway | class=`gke-l7-regional-external-managed`, listener `http:80`, allowedRoutes=Same namespace |
| HTTPRoute | matches `PathPrefix /`, backendRef `notiflex-api:80` weight=1 |
| HealthCheckPolicy | HTTP :8080 `/health`, checkInterval 15s, timeout 5s, healthyThreshold 1, unhealthyThreshold 2 |

### 3.3 Valkey (Helm values, 사용자 지정만)

```yaml
architecture: standalone
primary:
  resourcesPreset: none
  resources:
    requests: { cpu: 10m, memory: 64Mi }
    limits:   { cpu: 200m, memory: 128Mi }
```
- Service: `valkey-primary.notiflex.svc.cluster.local:6379` (ClusterIP)
- Secret: `valkey` (Helm 자동 생성, key=`valkey-password`) — **앱은 이 K8s Secret을 사용하지 않고 CSI 파일 마운트만 사용**
- PVC: 8Gi filesystem (AOF 영속성)

### 3.4 GCP IAM (실측)

**GCP SA `notiflex-secret-reader@git-ai-ops-practice.iam`에 부여된 권한**
- Secret `valkey-password` 접근: `roles/secretmanager.secretAccessor`

**이 GCP SA에 대한 impersonation 허용**
- Member: `serviceAccount:git-ai-ops-practice.svc.id.goog[notiflex/notiflex-api]`
- Member: `serviceAccount:git-ai-ops-practice.svc.id.goog[enterprise/notiflex-api]` (ch7.4 추가)
- Role: `roles/iam.workloadIdentityUser`

**K8s SA `notiflex/notiflex-api` annotation**
- `iam.gke.io/gcp-service-account: notiflex-secret-reader@git-ai-ops-practice.iam.gserviceaccount.com`

### 3.5 SecretProviderClass

```yaml
provider: gke
parameters:
  secrets: |
    - resourceName: "projects/git-ai-ops-practice/secrets/valkey-password/versions/latest"
      path: "valkey-password"
```

### 3.6 관측 스택 리소스 (축소된 상태)

| 컴포넌트 | CPU requests | Memory limits | 비고 |
|---------|-------------|---------------|------|
| Prometheus | 5m | 512Mi | scrape 30s, retention 7d |
| Grafana | 5m | 512Mi | sidecar (dashboards + datasources) |
| Alertmanager | 5m | 128Mi | receiver=null |
| kube-prometheus-operator | 5m | 128Mi | |
| kube-state-metrics | 10m | 64Mi | |
| node-exporter | 10m | 64Mi | DaemonSet |

### 3.7 ArgoCD App of Apps 구조

| Application | path | dest namespace | sync-wave | 관리 대상 |
|-------------|------|----------------|-----------|-----------|
| `root-app` | `argocd/apps` (directory.recurse) | `argocd` | (수동 부트스트랩) | 하위 Application들 자체 |
| `notiflex-kafka` | `k8s/kafka` | `kafka` | `1` (플랫폼) | KafkaNodePool, Kafka(v1, KRaft), KafkaTopic `notifications` (3개, `ServerSideApply=true`) |
| `notiflex-smb` | `k8s/smb` | `notiflex` | `2` (앱) | Namespace, Service×2, Rollout, Gateway, HTTPRoute, HealthCheckPolicy, SA, SPC, **CronJob** (10개) |
| `notiflex-enterprise` | `k8s/enterprise` | `enterprise` | `2` (앱) | Rollout, Service×2, SA, SecretProviderClass, ResourceQuota (6개, Namespace는 CreateNamespace=true 위임) |

**sync-wave 규약** (`argocd/root-app.yaml` 상단에 코드화)
- `0` = 인프라 (Gateway API/전역 Namespace/Quota 등)
- `1` = 플랫폼 (monitoring, messaging, tracing — 향후)
- `2` = 애플리케이션 (테넌트 워크로드)

모든 하위 Application은 `syncPolicy.automated.{prune,selfHeal}=true`, `syncOptions=[CreateNamespace=true]`.

### 3.8 CI (GitHub Actions `build-and-push`)

| 요소 | 값 |
|------|-----|
| 트리거 | `push` on `main`, paths=`app/**`, `.github/workflows/ci.yaml`; `workflow_dispatch` |
| concurrency | `build-and-push-${{ github.ref }}`, cancel-in-progress |
| 이미지 태그 | `sha-${GITHUB_SHA::7}` (예: `sha-954b417`) |
| 인증 | `GCP_SA_KEY` secret로 gcloud auth |
| 후속 job | rollout.yaml image line sed → github-actions[bot] 커밋 "[skip ci]" |

---

## 4. 현재 배포 상태 (요약)

- **앱**: notiflex-api v0.8.0 (`sha-0158fed`), Canary, replicas=1, Healthy — SMB(notiflex ns) + Enterprise(enterprise ns) 두 테넌트에 동시 배포, 둘 다 api-pool 위. Kafka Producer/Consumer + OTel 트레이싱 통합
- **캐시**: Valkey 9.1.2 (notiflex ns), `notiflex:id` 카운터를 SMB·Enterprise 두 테넌트가 cross-namespace DNS로 공유
- **메시징**: Kafka 4.3.0 KRaft 단일 브로커 (kafka ns, worker-pool), `notifications` 토픽 3파티션. /id 요청마다 3파티션에 라운드로빈 publish → 백그라운드 Consumer가 즉시 consume
- **트레이싱**: Tempo monolithic (monitoring ns, ops-pool), OTLP gRPC :4317. Grafana에서 `/id` 요청 하나가 4-span waterfall(GET /id → valkey.incr → kafka.produce → kafka.consume)로 조회됨
- **스케줄러**: `notiflex-healthcheck` CronJob (notiflex ns, ops-pool), 5분마다 `/health` 200 검증
- **시크릿**: `valkey-password` GSM v1 → CSI로 두 ns 모두 `/mnt/secrets/valkey-password` 마운트 (WI 바인딩 확장)
- **노드**: 5개 (default×2, api×1, worker×1, ops×1) — 모두 Spot
- **외부 IP**: 35.216.118.49 (notiflex-gateway, 현재는 SMB 테넌트만 노출)
- **ArgoCD Applications**: `root-app`, `notiflex-kafka`, `notiflex-smb`, `notiflex-enterprise` 모두 Synced/Healthy

## 5. 앞으로 예정된 구조 변화

| 챕터 | 변화 |
|------|------|
| ch9 | 저장소·클러스터 회고, 온보딩 문서 작성, GitAIOps 연결 분석, 다음 단계 제안 |
| (미정) | Loki/Fluent Bit 복원을 wave=1 플랫폼 App으로 편입 (ops-pool 활용) |
| (미정) | valkey/kube-prometheus/strimzi/tempo helm 릴리스를 wave=1 플랫폼 App으로 ArgoCD 관리에 통합 |
| (미정) | Argo Rollouts Gateway API 트래픽 라우터 플러그인 도입 → Canary weight의 실제 트래픽 분할 |
| (미정) | Alertmanager receiver 실 연결(Slack/OpsGenie 등), `kube_job_status_failed` 알림 편입 |
