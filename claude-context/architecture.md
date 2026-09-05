# Notiflex 아키텍처 스냅샷

**작성 시점**: 2026-09-05 (ch6.4 기준)
**책 진도**: ch6 완료 (캐시·시크릿·Canary 도입)

## 3층 지식 구조

이 저장소의 문서는 성격별로 3개 층으로 분리한다. 겹치는 정보는 두지 않는다.

| 문서 | 성격 | 시제 | 갱신 시점 | 로드 방식 |
|------|------|------|----------|----------|
| `CLAUDE.md` | 프로젝트 메타데이터·행동 규칙 | 상시 유효 | 규칙이 바뀔 때 | 매 대화 자동 로드 |
| `claude-context/` | 현재 아키텍처 스냅샷 | **현재** | 큰 구조 변화 후 | AI가 필요 시 참조 |
| `docs/architecture-decisions.md` | ADR (결정 누적) | **과거** | 결정 시점마다 append | AI/사람 참조용 |
| `JOURNEY.md` | 실제 진행·트러블슈팅 이력 | 시계열 | 각 서브챕터 완료 시 | AI가 필요 시 참조 |

핵심 구분:
- **왜 이 결정을 내렸는가** → `docs/architecture-decisions.md` (ADR)
- **지금 어떻게 동작하는가** → `claude-context/architecture.md` (이 파일)
- **어떤 문제를 겪었고 어떻게 풀었는가** → `JOURNEY.md`

## 클러스터 토폴로지

| 항목 | 값 |
|------|-----|
| 클러스터 이름 | `notiflex-cluster` |
| 리전/존 | `asia-northeast3` / `asia-northeast3-a` (Zonal) |
| GKE 모드 | Standard |
| 노드풀 | `default-pool` (e2-medium, Spot, 2 노드) |
| Workload Identity | `git-ai-ops-practice.svc.id.goog` (ch6.2 활성화) |
| Secret Manager CSI addon | Enabled (ch6.2, driver=`secrets-store-gke.csi.k8s.io`) |
| Gateway API | GKE managed (`gke-l7-regional-external-managed`) |
| kubectl context | `gke-sysnet4admin_book_gitaiops` |
| Artifact Registry | `asia-northeast3-docker.pkg.dev/git-ai-ops-practice/notiflex` |

**제약사항 (ch6 임시)**
- Rollout `replicas=1` (기본 2 → ch6.1에서 축소, ch7.2 노드풀 추가 후 복원 예정)
- Loki + Fluent Bit 임시 uninstall 상태 (ch6.2 CSI DaemonSet 240m 확보 목적, ch7.2에서 복원 예정)
- notiflex-api / Valkey CPU requests 10m로 재축소 (limits 200m는 유지)

## 컴포넌트 다이어그램

```
                                       ┌────────────────────────────────────┐
                                       │  GCP Secret Manager                │
                                       │    valkey-password (v1)            │
                                       └────────────────┬───────────────────┘
                                                        │ IAM: secretAccessor
                                                        │ (Workload Identity)
                                                        ▼
[외부 사용자]                         ┌───────────────────────────────────────┐
     │                                │  notiflex namespace                   │
     │ HTTP :80                       │                                       │
     ▼                                │   ┌─────────────────────────────┐    │
┌────────────────────────┐            │   │ ServiceAccount: notiflex-api │    │
│ Gateway (GKE L7 GCLB)   │            │   │ (WI → GCP SA)                │    │
│ notiflex-gateway        │            │   └─────────────┬────────────────┘    │
│ 35.216.118.49           │            │                 │                     │
└──────────┬─────────────┘            │                 ▼                     │
           │                          │   ┌─────────────────────────────┐    │
           ▼                          │   │ Rollout: notiflex-api        │    │
┌────────────────────────┐            │   │  strategy: Canary            │    │
│ HTTPRoute              │            │   │  steps: 20/50/80/100 (30s)   │    │
│ notiflex-route         │◀───────────┼───│  image: api:sha-954b417       │    │
└──────────┬─────────────┘            │   │                              │    │
           │                          │   │  volumeMounts:               │    │
           ▼                          │   │    /mnt/secrets ←────────────┼──┐ │
┌────────────────────────┐            │   │                              │  │ │
│ Service: notiflex-api  │─────────▶ Pod (v0.6.0)                        │  │ │
│  (stable, Canary)      │            │   │   │                          │  │ │
│                        │            │   │   │ INCR notiflex:id         │  │ │
│ Service: ...-preview   │─────────▶ Pod (canary)                        │  │ │
│  (canary, Canary)      │            │   │   │                          │  │ │
└────────────────────────┘            │   │   ▼                          │  │ │
                                       │   │  StatefulSet: valkey         │  │ │
                                       │   │  valkey-primary-0            │  │ │
                                       │   │  Service: valkey-primary:6379│  │ │
                                       │   └──────────────────────────────┘  │ │
                                       │                                     │ │
                                       │   ┌─────────────────────────────┐  │ │
                                       │   │ SecretProviderClass          │  │ │
                                       │   │ notiflex-secrets             │──┘ │
                                       │   │  (CSI driver=gke)            │    │
                                       │   └─────────────────────────────┘    │
                                       └───────────────────────────────────────┘
```

트래픽·상태·시크릿의 3가지 흐름:
- **트래픽**: 외부 → Gateway → HTTPRoute → Service → Rollout Pod
- **상태**: Rollout Pod → `INCR notiflex:id` → Valkey StatefulSet
- **시크릿**: GCP Secret Manager → CSI Driver(WI 인증) → Pod의 `/mnt/secrets/valkey-password` 파일

## 배포 파이프라인

```
┌──────────┐   git push    ┌─────────────────┐   docker push    ┌──────────────────┐
│ 개발자   │ ─────────────▶│ GitHub Actions  │────────────────▶│ Artifact Registry│
│          │  (app/**)     │  build-and-push │  api:sha-XXXXXXX │                  │
└──────────┘               └────────┬────────┘                  └──────────────────┘
                                    │
                                    │ CI 두 번째 job (update-manifest)
                                    │  - k8s/smb/rollout.yaml image line sed
                                    │  - 자동 커밋 [skip ci]
                                    ▼
                           ┌────────────────────┐
                           │ Git: main 브랜치    │
                           │  (매니페스트 갱신)  │
                           └────────┬───────────┘
                                    │ ArgoCD 3분 auto-sync
                                    │ 또는 hard refresh
                                    ▼
                           ┌────────────────────┐   Rollout controller
                           │ ArgoCD Application │ ────────────────────▶ 새 ReplicaSet 생성
                           │  notiflex-smb      │                       Canary 진행
                           │  (Synced/Healthy)  │                       (20→50→80→100)
                           └────────────────────┘
```

관리 경계:
- **ArgoCD 관리**: `k8s/smb/` (Namespace, Service×2, Rollout, Gateway, HTTPRoute, HealthCheckPolicy, ServiceAccount, SecretProviderClass)
- **Helm 직접 관리** (ArgoCD 밖): `valkey`, `kube-prometheus`
- **GKE 시스템 관리**: CSI Secrets Store DaemonSet, Workload Identity metadata server, GKE Managed Prometheus (`gmp-system`)

**ch7.3 App of Apps에서 helm 릴리스들도 ArgoCD가 관리하도록 통합 예정.**

## 관측 가능성

| 계층 | 도구 | 상태 | Namespace | 비고 |
|------|------|------|-----------|------|
| **메트릭** | Prometheus | Running | monitoring | kube-prometheus-stack, CPU 5m로 축소 |
| **대시보드** | Grafana | Running | monitoring | Loki/Prometheus datasource sidecar, memory limit 512Mi |
| **알림** | Alertmanager | Running | monitoring | PrometheusRule 3개(pod-restart 등), receiver=null |
| **로그** | Loki + Fluent Bit | **임시 제거** | monitoring | ch6.2 CSI CPU 확보를 위해 uninstall, ch7.2 복원 예정 |
| **트레이싱** | Tempo | 미도입 | - | ch8.2에서 도입 예정 |

**대체 확인 수단 (Loki 부재 기간)**
- 앱 로그: `kubectl --context gke-sysnet4admin_book_gitaiops logs -n notiflex -l app=notiflex-api`
- 메트릭: Grafana 대시보드 (Prometheus datasource 유지)
- 알림: Alertmanager UI (`kubectl port-forward svc/kube-prometheus-kube-prome-alertmanager 9093:9093 -n monitoring`)

## 주요 네임스페이스

| Namespace | 주요 워크로드 | 관리 주체 |
|-----------|-------------|----------|
| `notiflex` | Rollout(notiflex-api), StatefulSet(valkey-primary), Gateway, HTTPRoute, HealthCheckPolicy, ServiceAccount, SecretProviderClass | ArgoCD(`notiflex-smb`) + Helm(valkey) |
| `monitoring` | kube-prometheus-stack (Prometheus, Grafana, Alertmanager, kube-state-metrics, operator, node-exporter) | Helm(kube-prometheus) |
| `argocd` | argocd-server, application-controller, repo-server, redis, dex, notifications, applicationset | 수동 설치(ch3.2 stable manifest) |
| `argo-rollouts` | argo-rollouts controller(v1.10.0) | 수동 설치(--server-side, ch5.3) |
| `kube-system` | CSI Secrets Store DaemonSet(GKE managed), Workload Identity metadata server, kube-dns, fluentbit-gke(GKE 자체), kube-proxy 등 | GKE 관리 |
| `gmp-system` | GKE Managed Prometheus collector·operator | GKE 관리 (ch6.2 secret-manager 활성화 시 함께 딸려옴) |
| `gke-managed-*` | GKE 관리 시스템 컴포넌트 | GKE 관리 |

## 현재 배포 상태 (요약)

- **앱**: notiflex-api v0.6.0 (`sha-954b417`), Canary strategy, replicas=1, Healthy
- **캐시**: Valkey standalone v9.1.2, `valkey-primary:6379`, 카운터 keys=`notiflex:id`
- **시크릿**: `valkey-password`가 GCP Secret Manager v1에 저장, CSI로 `/mnt/secrets/valkey-password`에 마운트
- **외부 IP**: 35.216.118.49 (Gateway, HTTP:80)
- **ArgoCD Application**: `notiflex-smb` (Synced/Healthy)

## 앞으로 예정된 구조 변화

- **ch7.2 멀티 노드풀**: 역할별(api/worker/ops) 노드풀 분리 → Loki/FluentBit 복원 + Rollout replicas 2 복원
- **ch7.3 App of Apps**: 상위 Application으로 helm 릴리스들(valkey, kube-prometheus, loki, fluent-bit)을 통합 관리
- **ch7.4 멀티테넌시**: 테넌트별 Namespace 분리
- **ch8.1 메시징**: Kafka(Strimzi) 도입
- **ch8.2 트레이싱**: Tempo + OTel SDK
- **ch8.3 CronJob**: 헬스체크 CronJob
