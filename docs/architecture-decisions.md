# Architecture Decision Records

## ADR-001: GitOps 도구로 ArgoCD 채택 (3장)
**시점**: 2026-09 / **결정**: ArgoCD를 GitOps 도구로 채택. Flux, Jenkins X, Spinnaker는 사용하지 않는다.
**이유**:
- **Web UI**로 배포 상태를 실시간 확인 → 학습 과정에서 "지금 무슨 일이 일어나는지" 눈으로 볼 수 있다
- **Application CRD**로 "어떤 Git 경로 → 어떤 네임스페이스" 선언적 관리
- **selfHeal**: 누군가 `kubectl edit`으로 직접 수정해도 Git 상태로 되돌린다
- GKE Standard와 네이티브 호환, e2-medium 노드에서 구동 가능 (~500MB 메모리)

## ADR-002: CI 도구로 GitHub Actions 채택 (3장)
**시점**: 2026-09 / **결정**: GitHub Actions를 CI로 채택. Cloud Build, GitLab CI, Jenkins는 사용하지 않는다.
**이유**:
- **GitHub 네이티브**: 코드 저장소와 CI가 같은 플랫폼 → 별도 서버 설치/관리 불필요
- **YAML 선언적**: `.github/workflows/ci.yaml` 한 파일로 파이프라인 정의
- **무료 크레딧**: 퍼블릭 저장소 무제한, 프라이빗도 월 2,000분 무료
- **GCP 인증 간편**: `google-github-actions/auth` 액션으로 서비스 계정 키 한 줄 연동

## ADR-003: 메트릭 모니터링으로 Prometheus + Grafana (kube-prometheus-stack) 채택 (4장)
**시점**: 2026-09 / **결정**: kube-prometheus-stack Helm 번들로 Prometheus + Grafana + Alertmanager + node-exporter + kube-state-metrics를 한 번에 도입. Datadog, CloudWatch, GCP Monitoring은 사용하지 않는다.
**이유**:
- **오픈소스 표준**: Kubernetes 모니터링의 사실상 표준 (CNCF Graduated)
- **비용 없음**: SaaS 구독료 없이 자체 호스팅
- **Helm 번들**: 6개 컴포넌트를 검증된 버전 조합으로 한 번에 설치
- **Grafana**: 대시보드로 시각적 모니터링, 4장 후반에서 Loki/Tempo와 연결

## ADR-004: 로그 수집으로 Loki + Fluent Bit 채택 (4장)
**시점**: 2026-09 / **결정**: Loki(SingleBinary)와 공식 fluent/fluent-bit DaemonSet 조합을 채택. ELK, CloudWatch Logs, GCP Logging은 사용하지 않는다.
**이유**:
- **경량**: Loki 128Mi, Fluent Bit 64Mi — e2-medium에서 ELK(2Gi+)는 불가능
- **Grafana 통합**: 메트릭(Prometheus)과 같은 UI에서 로그를 조회
- **라벨 기반 인덱싱**: 풀텍스트 인덱싱 대비 저장 비용이 낮다
- **GitOps 호환**: helm-values를 저장소에 커밋해 ArgoCD가 동기화 (grafana/fluent-bit 차트는 deprecated + image override 불가로 배제)

## ADR-005: 알림 방식으로 PrometheusRule + Alertmanager 채택 (4장)
**시점**: 2026-09 / **결정**: Prometheus Operator의 `PrometheusRule` CRD와 Alertmanager를 알림 파이프라인으로 채택. Grafana Alerting, PagerDuty, GCP Cloud Monitoring은 사용하지 않는다.
**이유**:
- **GitOps 호환**: `PrometheusRule` CRD를 매니페스트로 관리한다. Git 저장소에 규칙이 누적되고 ArgoCD가 클러스터에 적용한다
- **이미 설치됨**: ADR-003의 kube-prometheus-stack 설치 시 Alertmanager + PrometheusRule CRD가 함께 들어온다 → 추가 설치 불필요
- **표준 도구**: 실무에서 가장 보편적인 알림 스택. 책 전반의 GitOps 우선 원칙과 일치
- **강력한 라우팅**: Alertmanager의 그루핑/억제/라우팅 트리가 다단계 알림을 표현한다

## ADR-006: 외부 트래픽 관리로 Gateway API 채택 (5장)
**시점**: 2026-09 / **결정**: GKE의 GatewayClass `gke-l7-regional-external-managed`를 사용하는 Gateway API v1을 채택. Ingress NGINX, Istio, Traefik은 사용하지 않는다.
**이유**:
- **K8s 공식 표준**: Ingress를 대체하는 차세대 API (GA since K8s 1.27)
- **GKE 네이티브**: 별도 Ingress Controller 설치 없이 GKE가 자동으로 처리 (컨트롤러 리소스 0)
- **역할 분리**: Gateway(인프라팀) / HTTPRoute(앱팀)로 관심사 분리 → 앱 추가 시 Gateway 미수정
- **5.3 Blue/Green 연동**: HTTPRoute의 backendRefs로 트래픽 분배 가능. GKE 전용 `HealthCheckPolicy` CRD로 `/health:8080` 프로브 지정하여 기본 `/` 프로브의 `no healthy upstream` 회피

## ADR-007: 무중단 배포 전략으로 Argo Rollouts Blue/Green 채택 (5장)
**시점**: 2026-09 / **결정**: Argo Rollouts를 배포 컨트롤러로 도입하고 Blue/Green 전략을 사용한다. Flagger, K8s native Rolling Update는 사용하지 않는다. Canary 전환은 6.3에서 예정.
**이유**:
- **ArgoCD 통합**: 같은 Argo 프로젝트, ArgoCD UI에서 Rollout 상태를 확인 가능
- **CRD 기반**: YAML 선언으로 배포 전략 정의, GitOps 호환
- **점진적 진화**: 5장 Blue/Green → 6장 Canary로 전략을 바꿀 때 Rollout CRD의 `strategy` 필드만 수정하면 됨
- **kubectl 플러그인**: `kubectl argo rollouts status`로 배포 진행 상태를 실시간 모니터링. active/preview Service 분리로 QA 창구 확보

## ADR-008: 캐시/상태 공유 저장소로 Valkey 채택 (6장)
**시점**: 2026-09 / **결정**: Pod 간 상태 공유를 위해 Valkey(standalone)를 채택하고 Redis, Memcached, DragonflyDB는 채택하지 않음
**이유**:
- **라이선스 안전**: Redis는 2024년 SSPL로 전환되어 상용 서비스에 제약이 생겼지만 Valkey는 Linux Foundation 산하 BSD 라이선스라 상용 리스크 없음
- **Redis 완전 호환**: 명령어·클라이언트가 100% 동일해서 기존 Redis 지식·라이브러리(valkey-go)를 그대로 재사용, 학습 곡선 0
- **INCR + 영속성 동시 만족**: `/id` 엔드포인트가 원자적 카운터를 요구하고 재시작 후에도 유지되어야 함. Memcached는 영속성 부재로 부적합
- **Bitnami Helm 차트 성숙**: `helm install valkey bitnami/valkey`로 standalone 즉시 배포, CPU 10m만으로 e2-medium 노드에서 감당 가능

## ADR-009: 시크릿 관리로 GKE Secret Manager CSI + Workload Identity 채택 (6장)
**시점**: 2026-09 / **결정**: 시크릿을 GCP Secret Manager에 저장하고 Secrets Store CSI Driver(GKE addon)로 Pod에 파일 마운트, Workload Identity로 SA 키 없이 인증. Sealed Secrets, External Secrets Operator, kubectl K8s Secret은 채택하지 않음
**이유**:
- **GKE 네이티브 통합**: `--enable-secret-manager` addon만으로 CSI Driver 배포 완료, 별도 Operator 유지·업그레이드 부담 없음
- **SA 키 파일 불필요**: Workload Identity로 K8s SA ↔ GCP SA를 OIDC 토큰 방식으로 매핑, JSON 키 파일 배포·회수·유출 리스크 제거
- **원본 단일화**: Secret Manager가 유일한 진실 소스이며 GCP의 감사 로그·자동 회전·IAM 기반 접근 제어를 그대로 활용
- **GitOps 호환**: SecretProviderClass·ServiceAccount만 Git에 커밋하면 실제 비밀번호는 저장소에 남지 않음. Sealed Secrets는 개인키 관리 부담, ESO는 별도 Operator 필요

## ADR-010: 배포 전략을 Blue/Green에서 Canary로 진화 (6장)
**시점**: 2026-09 / **결정**: Argo Rollouts의 `strategy` 필드를 `blueGreen`에서 `canary`(steps 20/50/80/100, 각 30s pause)로 교체. Blue/Green 유지, Flagger, Istio는 채택하지 않음
**이유**:
- **문제 영향 범위 최소화**: Blue/Green의 0%→100% 즉시 전환은 promote 직후 오류가 전체 사용자에게 노출됨. Canary의 점진 전환은 첫 단계에 소수 사용자만 영향받고 abort로 즉시 회수 가능
- **리소스 절감**: Blue/Green은 항상 2x 리소스 필요, Canary는 약 1.2x로 e2-medium 2노드 제약 환경에 더 적합 (ch6 CSI 도입 이후 CPU가 특히 빠듯)
- **도구 변경 없음**: 5.3에서 이미 도입한 Argo Rollouts 그대로 유지, Rollout CRD의 `strategy` 필드만 교체. Flagger는 Flux 생태계라 이중 관리, Istio는 서비스 메시 도입 부담이 큼
- **점진적 진화 학습**: Rolling → Blue/Green → Canary로 같은 도구 위에서 전략을 진화시키는 이 책의 핵심 패턴을 완성

## ADR-011: 워크로드별 노드 배치를 nodeSelector + 멀티 노드풀로 결정 (7장)
**시점**: 2026-09 / **결정**: `api-pool`(e2-medium)·`worker-pool`(e2-standard-2)·`ops-pool`(e2-small) 3개 노드풀을 신설하고 Pod의 `nodeSelector: cloud.google.com/gke-nodepool=<pool>` 로 배치. taint/toleration, nodeAffinity, topology spread는 채택하지 않음
**이유**:
- **최소 학습 곡선**: 라벨 매칭 한 줄만 추가하면 되어 taint/toleration의 이중 설정 부담이나 nodeAffinity의 표현식 학습이 불필요
- **GKE 자동 라벨 활용**: 노드풀 생성 시 `cloud.google.com/gke-nodepool` 라벨이 자동 부여되어 별도 라벨 작업이 없고, 잘못된 커스텀 키 사용으로 인한 Pod Pending 사고를 원천 차단
- **리소스 격리**: Kafka(예정) 같은 메모리 대량 워커가 API Pod의 자원을 잠식하는 상황을 물리 분리로 해결하여 서비스 안정성 확보
- **단일 존 환경 적합**: `asia-northeast3-a` 단일 존이라 topology spread의 AZ 분산 이점이 없고, vCluster/별도 클러스터는 5노드 규모에 과함

## ADR-012: 다중 앱 관리로 App of Apps 패턴 채택 (7장)
**시점**: 2026-09 / **결정**: `argocd/root-app.yaml`이 `argocd/apps/` 디렉터리를 `directory.recurse: true`로 감시하고, sync-wave 규약(0=인프라 / 1=플랫폼 / 2=애플리케이션)으로 순서를 강제. ApplicationSet과 수동 관리는 채택하지 않음
**이유**:
- **순수 YAML, 문법 학습 없음**: 템플릿 문법 없이 Application 매니페스트를 폴더에 넣기만 하면 앱이 생성되어 학습·디버깅 부담이 최소
- **GitOps 원칙 충실**: "폴더에 YAML 추가 = 앱 추가" 흐름으로 사람의 `kubectl apply` 개입을 제거하고 Git이 유일한 진실 소스가 됨
- **sync-wave로 의존성 명시**: 앞 wave가 Healthy 되어야 다음 wave가 시작되어 인프라·플랫폼·앱 순서를 매니페스트로 표현 가능. 별도 대기 로직 불필요
- **규모 적합**: 관리 대상 앱이 5~7개 수준이라 ApplicationSet의 Generator 기반 대량 생성 이점이 없고, 단일 클러스터라 다중 환경 템플릿 필요성도 없음

## ADR-013: 멀티테넌시로 Namespace 분리 + per-tenant Rollout 채택 (7장)
**시점**: 2026-09 / **결정**: 테넌트별 Namespace(예: `enterprise`) + 전용 Rollout/Service/SA/SecretProviderClass + ResourceQuota. Valkey 같은 공유 인프라는 cross-namespace DNS로 접근. 단일 namespace + 라벨 격리, NetworkPolicy 추가, vCluster, 클러스터별 분리는 채택하지 않음
**이유**:
- **K8s 기본 기능만으로 즉시 격리**: Namespace + RBAC + ResourceQuota는 추가 도구 설치 없이 K8s 표준 API로 구성 가능하여 학습·운영 부담이 최소
- **기존 자산 재활용**: ch6.2 CSI+WI(테넌트 SA를 동일 GSA에 추가 바인딩), ch7.2 api-pool nodeSelector, ch7.3 App of Apps + sync-wave 를 그대로 결합하여 새 도구 도입 없이 온보딩 완료
- **공유 인프라 실전 학습**: `valkey-primary.notiflex.svc.cluster.local` 형태의 cross-namespace DNS로 공유 자원을 재사용하여 리소스 중복 없이 실제 SaaS 패턴 경험
- **격리 강도 vs 비용 균형**: 단일 e2-medium × 5노드 클러스터에서 vCluster는 최소 100~300MB 오버헤드, 별도 클러스터는 비용 2배+. NetworkPolicy는 Dataplane V2 재구성이 필요해 학습 단계 범위 밖. ResourceQuota로 노이지 네이버는 완화
