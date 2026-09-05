# Notiflex 아키텍처 스냅샷

**작성 시점**: 2026-09-05 (ch6.4)
**책 진도**: ch6 완료 (캐시·시크릿·Canary 도입)
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
| 노드풀 | `default-pool` — e2-medium, **Spot**, 2 노드 |
| Workload Identity | `git-ai-ops-practice.svc.id.goog` (ch6.2 활성화) |
| Secret Manager CSI | GKE addon, driver=`secrets-store-gke.csi.k8s.io` |
| Gateway API | GKE managed `gke-l7-regional-external-managed` |
| Artifact Registry | `asia-northeast3-docker.pkg.dev/git-ai-ops-practice/notiflex` |
| kubectl context | `gke-sysnet4admin_book_gitaiops` |

### 1.2 애플리케이션 (notiflex namespace)

| 컴포넌트 | 종류 | 관리 | 상세 |
|---------|------|------|------|
| `notiflex-api` | Rollout (Canary) | ArgoCD | image `api:sha-954b417` (v0.6.0), replicas=1, SA=`notiflex-api` |
| `valkey-primary` | StatefulSet | Helm | chart `bitnami/valkey-6.2.19` (app 9.1.2), standalone, replicas=1 |
| `notiflex-secrets` | SecretProviderClass | ArgoCD | provider=`gke`, ref=`projects/.../secrets/valkey-password/versions/latest` |
| `notiflex-api` (Service) | ClusterIP | ArgoCD | stable Service — Canary의 stableService, port 80→8080 |
| `notiflex-api-preview` | ClusterIP | ArgoCD | canary Service — Canary의 canaryService |
| `notiflex-gateway` | Gateway v1 | ArgoCD | classes=`gke-l7-regional-external-managed`, HTTP:80, IP=35.216.118.49 |
| `notiflex-route` | HTTPRoute | ArgoCD | `/` prefix → `notiflex-api:80` (weight=1) |
| `notiflex-healthcheck` | HealthCheckPolicy | ArgoCD | HTTP :8080 `/health`, interval 15s, threshold 2/1, timeout 5s |
| `notiflex-api` (SA) | ServiceAccount | ArgoCD | WI annotation → GCP SA `notiflex-secret-reader` |

### 1.3 배포·GitOps (argocd, argo-rollouts namespaces)

| 컴포넌트 | 종류 | 관리 | 상세 |
|---------|------|------|------|
| ArgoCD (7종) | Deployment/STS | 수동 설치 | v3.5.2, Application `notiflex-smb` 하나만 등록 (path=`k8s/smb`) |
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
| **Loki** | **임시 제거** | ch6.2 CSI CPU 확보용, ch7.2에서 복원 |
| **Fluent Bit** | **임시 제거** | ch6.2 CSI CPU 확보용, ch7.2에서 복원 |
| Tempo | 미도입 | ch8.2 예정 |

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
- **ArgoCD 소유**: `k8s/smb/` 내 모든 매니페스트 (9개 리소스)
- **Helm 소유** (ArgoCD 밖): `valkey` (notiflex), `kube-prometheus` (monitoring)
- **GKE 시스템**: CSI Secrets Store DaemonSet, gke-metadata-server, kube-dns, GKE Managed Prometheus (gmp-system)

**ch7.3 App of Apps에서 helm 릴리스들도 ArgoCD 관리로 통합 예정.**

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
| container image | `.../api:sha-954b417` (v0.6.0) |
| readinessProbe | HTTP :8080 `/health`, initialDelay 2s, period 5s |
| livenessProbe | HTTP :8080 `/health`, initialDelay 5s, period 10s |
| resources.requests | cpu 10m, memory 32Mi |
| resources.limits | cpu 200m, memory 128Mi |
| serviceAccountName | notiflex-api |
| env | `VALKEY_ADDR=valkey-primary.notiflex.svc.cluster.local:6379`, `VALKEY_PASSWORD_FILE=/mnt/secrets/valkey-password` |
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

### 3.7 ArgoCD Application (notiflex-smb)

| 필드 | 값 |
|------|-----|
| repoURL | `https://github.com/jheon-eom/notiflex-platform.git` |
| path | `k8s/smb` |
| targetRevision | `main` |
| destination | `https://kubernetes.default.svc`, namespace=`notiflex` |
| syncPolicy | automated (prune=true, selfHeal=true), CreateNamespace=true |
| 관리 리소스 수 | 9개 (Namespace, Service×2, Rollout, Gateway, HTTPRoute, HealthCheckPolicy, ServiceAccount, SecretProviderClass) |

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

- **앱**: notiflex-api v0.6.0 (`sha-954b417`), Canary, replicas=1, Healthy
- **캐시**: Valkey 9.1.2, `notiflex:id` 카운터 저장
- **시크릿**: `valkey-password` GSM v1 → CSI로 `/mnt/secrets/valkey-password` 마운트
- **외부 IP**: 35.216.118.49
- **ArgoCD Application**: `notiflex-smb` Synced/Healthy

## 5. 앞으로 예정된 구조 변화

| 챕터 | 변화 |
|------|------|
| ch7.2 | 역할별 노드풀 추가 → Loki/Fluent Bit 복원 + Rollout replicas 2 복원 |
| ch7.3 | App of Apps로 valkey·kube-prometheus 등 helm 릴리스를 ArgoCD 관리로 통합 |
| ch7.4 | 테넌트별 Namespace 분리 |
| ch8.1 | Kafka (Strimzi operator) 도입 |
| ch8.2 | Tempo + OTel SDK로 트레이싱 |
| ch8.3 | 헬스체크 CronJob |
