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
