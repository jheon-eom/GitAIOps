# Notiflex 여정 기록

이 파일은 독자가 실제로 진행한 내용을 기록한다. AI가 각 챕터 완료 시 자동으로 업데이트한다.

## 진행 현황

| 챕터 | 서브챕터 | 상태 | 완료일 | 비고 |
|------|---------|------|--------|------|
| ch2 | 2.2 설치 확인 | ✅ | 2026-09-03 | Claude Code CLI 준비 완료 (환경 재확인 없이 바로 2.3 진입) |
| ch2 | 2.3 gcloud 설정 | ✅ | 2026-09-03 | Homebrew cask로 설치, gke-gcloud-auth-plugin 별도 추가 필요 |
| ch2 | 2.4 GitHub 저장소 | ✅ | 2026-09-03 | jheon-eom/notiflex-platform (private) |
| ch2 | 2.5 GKE 클러스터 | ✅ | 2026-09-03 | notiflex-cluster (Zonal, Gateway API 활성) |
| ch2 | 2.6 빌드/배포 | ✅ | 2026-09-03 | Cloud Build로 이미지 빌드, Spot 노드 preempt 1회 겪음 |
| ch2 | 2.7 첫 커밋 | ✅ | 2026-09-03 | JOURNEY.md 생성 및 최초 push |
| ch3 | 3.2 GitOps 도구 | ✅ | 2026-09-04 | ArgoCD v3.5.2 설치, notiflex-smb Application Synced/Healthy |
| ch3 | 3.3 기능 추가 | ✅ | 2026-09-04 | /version 엔드포인트 추가, v0.1.1 Rolling Update 완료 |
| ch3 | 3.4 CI | ✅ | 2026-09-04 | GitHub Actions로 app/** 변경 시 이미지 자동 빌드·push, SHA 태그(sha-xxxxxxx) 방식 |
| ch3 | 3.5 CI-CD 연결 | ✅ | 2026-09-04 | CI가 매니페스트 자동 커밋([skip ci]), ArgoCD가 감지하여 자동 배포. /ping E2E 검증 완료 |
| ch4 | 4.2 메트릭 모니터링 | ✅ | 2026-09-04 | kube-prometheus-stack Helm 설치, Grafana에 Notiflex 대시보드 ConfigMap 자동 로드 |
| ch4 | 4.3 로그 수집 | ✅ | 2026-09-04 | Loki SingleBinary + fluent/fluent-bit DaemonSet, Grafana에 Loki 데이터소스 자동 등록 (kubernetes_namespace_name 라벨) |
| ch4 | 4.4 알림 | ✅ | 2026-09-04 | PrometheusRule 3개(PodRestartTooMany, NotiflexHighCpu, 파이프라인 검증용) 배포, Alertmanager 수신 확인. 실 receiver는 null (Slack 미연결) |
| ch5 | 5.2 트래픽 관리 | ✅ | 2026-09-04 | GKE Gateway API 도입, 외부 IP 35.216.118.49로 /health·/version 200 OK 확인 |
| ch5 | 5.3 무중단 배포 | ✅ | 2026-09-04 | Argo Rollouts v1.10.0 도입, Deployment→Rollout 전환, v0.2.0 배포 시 Blue/Green auto-promote(30s) 실증 |
| ch5 | 5.4 ADR 기록 | ✅ | 2026-09-04 | docs/architecture-decisions.md 신설, ADR-001~007로 3~5장 결정 정리 |
| ch6 | 6.1 캐시 | ✅ | 2026-09-05 | Valkey standalone 도입, /id를 Valkey INCR로 이전. v0.4.0. 리소스 여유 확보 위해 replicas 2→1로 축소. Pod 재시작 후 카운터(11부터) 유지 확인 |
| ch6 | 6.2 시크릿 관리 | ✅ | 2026-09-05 | GCP Secret Manager + CSI Driver + Workload Identity 도입. v0.5.0. Valkey password를 GSM에 저장하고 Pod에 파일(/mnt/secrets/valkey-password)로 마운트. SA 키 없이 동작 |
| ch6 | 6.3 Canary 전환 | ⬜ | | |
| ch7 | 7.2 멀티 노드풀 | ⬜ | | |
| ch7 | 7.3 App of Apps | ⬜ | | |
| ch7 | 7.4 멀티테넌시 | ⬜ | | |
| ch8 | 8.1 메시징 | ⬜ | | |
| ch8 | 8.2 트레이싱 | ⬜ | | |
| ch8 | 8.3 CronJob | ⬜ | | |
| ch9 | 9.1 저장소 분석 | ⬜ | | |
| ch9 | 9.2 회고 | ⬜ | | |
| ch9 | 9.3 온보딩 문서 | ⬜ | | |
| ch9 | 9.4 GitAIOps 분석 | ⬜ | | |
| ch9 | 9.5 마무리 | ⬜ | | |

## 도구 선택 기록

독자가 3-프롬프트 패턴(탐색→비교→실행)에서 실제로 선택한 도구와 이유를 기록한다.

| 영역 | 선택 | 검토한 대안 | 선택 이유 |
|------|------|-----------|----------|
| GitOps 도구 | ArgoCD | Flux, Jenkins X, Spinnaker | Web UI로 배포 상태 시각화가 학습·실습에 유리, e2-medium 노드에서 감당 가능한 리소스(~500MB) |
| CI 도구 | GitHub Actions | Cloud Build, GitLab CI, Jenkins | 코드가 이미 GitHub에 있어 별도 서버·플랫폼 이동 불필요, YAML 한 파일로 파이프라인 정의 |
| 메트릭 모니터링 | Prometheus + Grafana (kube-prometheus-stack) | Datadog, CloudWatch, GCP Monitoring | 오픈소스 표준·비용 0원, Helm 번들로 6개 컴포넌트 일괄 설치, 이후 Loki/Tempo와 Grafana 하나로 통합 가능 |
| 로그 수집 | Loki + Fluent Bit (fluent/fluent-bit 차트) | ELK, CloudWatch Logs, GCP Logging | 경량(~200Mi 총합)으로 e2-medium 감당 가능, Grafana에서 메트릭·로그 동시 조회, 라벨 인덱싱으로 저장 비용 낮음 |
| 알림 방식 | PrometheusRule + Alertmanager | Grafana Alerting, PagerDuty, GCP Cloud Monitoring | GitOps 흐름 유지(YAML → Git → ArgoCD 동기화), 4.2에서 이미 설치돼 추가 비용 0, git blame으로 임계값 근거 추적 가능 |
| 외부 트래픽 관리 | Gateway API (gke-l7-regional-external-managed) | Ingress NGINX, Istio, Traefik | GKE 네이티브라 Controller 설치·유지 리소스 0, Gateway/HTTPRoute 역할 분리, 5.3 Blue/Green에서 backendRefs weight로 자연 확장 |
| 무중단 배포 전략 | Argo Rollouts (Blue/Green) | Flagger, K8s Rolling Update | ArgoCD와 같은 Argo 생태계로 UI 통합, Rollout CRD strategy만 교체하면 6장 Canary로 진화 가능, kubectl 플러그인으로 실시간 관찰 |
| 캐시/상태 공유 (ch6.1) | Valkey | Redis, Memcached, DragonflyDB | Redis 100% 호환 + BSD 라이선스로 상용 리스크 제거. Bitnami Helm 차트로 standalone 즉시 배포, CPU 50m만 사용. Redis는 SSPL 라이선스 부담, Memcached는 영속성 부재, DragonflyDB는 아직 미성숙 |
| 시크릿 관리 (ch6.2) | GKE Secret Manager CSI + Workload Identity | Sealed Secrets, External Secrets Operator, kubectl Secret | GKE 네이티브 통합으로 CSI addon 활성화만으로 도입, Workload Identity로 SA 키 파일 불필요, GCP Secret Manager가 원본(감사 로그·자동 회전 활용). Sealed Secrets는 클라우드 종속성 없지만 개인키 관리 부담, ESO는 별도 Operator 유지 필요 |

## 현재 버전

| 컴포넌트 | 버전 | 변경 이력 |
|---------|------|----------|
| Go | 1.25 | 2026-09-03 초기 설정 (ch6 valkey-go, ch8 OTel SDK 대비) |
| Notiflex 이미지 | sha-c17a1ed (v0.5.0) | 2026-09-05 6.2에서 valkey password를 파일로 로드. 이력: v0.1.0(수동) → v0.1.1(수동, /version) → sha-97380d1(CI, 최초 자동) → sha-d1462c9(CI, /ping E2E) → sha-11a274f(CI, 5.3 Blue/Green 첫 승격) → sha-afbc9b9(CI, 5.3 Blue/Green 관찰 시연) → sha-b1962d9(CI, 6.1 Valkey INCR 통합) → sha-c17a1ed(CI, 6.2 CSI Secret 통합) |
| ArgoCD | v3.5.2 | 2026-09-04 설치 (stable manifest) |
| kube-prometheus-stack | chart 89.2.0 (operator v0.93.1) / Prometheus v3.14.0 / Grafana 13.2.1 / Alertmanager v0.34.0 | 2026-09-04 설치. Prometheus 100m/256Mi, Alertmanager 25m/64Mi 초기값 (ch6 CSI 대비 임시). Grafana는 4.3에서 sidecar.datasources 활성화 + OOMKilled로 memory limit 256→512Mi 상향 |
| Loki | chart 7.3.0 / app 3.6.12 (SingleBinary) | 2026-09-04 설치. schemaConfig v13 명시, useTestSchema 제거, backend/read/write replicas=0 |
| Fluent Bit | chart 0.58.1 / app 5.1.1 (DaemonSet) | 2026-09-04 설치. grafana/fluent-bit는 deprecated + image override 불가로 공식 fluent/fluent-bit 차트로 전환. Read_from_Head On으로 기존 로그도 수집 |
| PrometheusRule | notiflex-alerts (3 rules) | 2026-09-04 4.4에서 pod-restart-alert.yaml 배포: PodRestartTooMany, NotiflexHighCpu, NotiflexAlertPipelineTest(검증용). Alertmanager receiver는 null (Slack 미연결) |
| Gateway API | Gateway v1 + HealthCheckPolicy v1 | 2026-09-04 5.2에서 도입. GatewayClass=gke-l7-regional-external-managed, 외부 IP=35.216.118.49. HealthCheckPolicy로 /health:8080 헬스체크 지정(기본 / 프로브 시 no healthy upstream 회피). proxy-only-subnet(asia-northeast3, 172.16.0.0/23) 신규 생성 |
| Argo Rollouts | controller v1.10.0 / plugin v1.9.0 | 2026-09-04 5.3에서 도입. install.yaml은 `--server-side` 필수(CRD annotation size 초과 회피). Rollout strategy=BlueGreen, autoPromotionSeconds=30. 앞으로 6.3 Canary 전환 시 strategy 필드만 교체. 6.1에서 replicas 2→1 축소(리소스 확보) |
| Valkey | Bitnami chart 6.2.19 / app 9.1.2 (standalone) | 2026-09-05 6.1에서 도입. architecture=standalone, resourcesPreset=none. 6.2에서 CPU 50m→10m로 재축소 (CSI DaemonSet 240m 추가에 대응). Service=`valkey-primary.notiflex.svc.cluster.local:6379` |
| valkey-go | v1.0.77 | 2026-09-05 Go valkey-go 클라이언트 도입. 10회×3s 재시도 로직으로 DNS/Valkey 기동 순서 이슈 방어 |
| Workload Identity | git-ai-ops-practice.svc.id.goog | 2026-09-05 6.2에서 클러스터+default-pool에 활성화 (5-10분 소요). K8s SA `notiflex/notiflex-api` ↔ GCP SA `notiflex-secret-reader@git-ai-ops-practice.iam` 매핑 |
| Secret Manager CSI (GKE managed) | driver=`secrets-store-gke.csi.k8s.io`, provider=`gke` | 2026-09-05 6.2에서 `--enable-secret-manager` 활성화. DaemonSet 2개(csi-secrets-store-gke, csi-secrets-store-provider-gke) 총 노드당 120m. |
| GCP Secret Manager | valkey-password (v1) | 2026-09-05 6.2에서 도입. Valkey 비밀번호가 원본으로 저장, IAM(roles/secretmanager.secretAccessor)으로 접근 제어 |
| SecretProviderClass | notiflex-secrets | 2026-09-05 6.2에서 배포. provider=gke, `projects/.../secrets/valkey-password/versions/latest`를 Pod의 `/mnt/secrets/valkey-password`로 마운트 |
| kube-prometheus-stack (ch6 축소) | 동일 chart | 2026-09-05 ch6 진입 전 리소스 축소. prometheus/grafana/alertmanager/operator cpu 각각 5m로 조정. ch7 노드풀 추가 후 원복 검토 |
| Loki (ch6.2 임시 제거) | 동일 chart | 2026-09-05 6.2에서 CSI DaemonSet 240m 추가로 CPU 부족 → 임시 uninstall. ch7.2 노드풀 추가 후 복원 예정 |
| Fluent Bit (ch6.2 임시 제거) | 동일 chart | 2026-09-05 6.2에서 CPU 여유 확보 위해 임시 uninstall. ch7.2 복원 예정. 앱 stdout 로그는 `kubectl logs`로 조회 가능 |
| Kafka | | |
| OTel SDK | | |

## 현재 리소스

| 노드풀 | 머신 타입 | 노드 수 | 주요 워크로드 |
|--------|----------|---------|-------------|
| default-pool | e2-medium (Spot, disk 30GB) | 2 | notiflex-api Rollout(1 replica, CPU 10m — ch6.2에서 재축소), Valkey standalone(CPU 10m), ArgoCD, kube-prometheus-stack(축소), CSI Secrets Store DaemonSet(GKE managed, 노드당 120m), argo-rollouts controller. **Loki/Fluent Bit는 ch6.2에서 임시 uninstall, ch7.2에서 복원 예정** |

**GCP 컨텍스트**
- Project: `git-ai-ops-practice`
- Region/Zone: `asia-northeast3` / `asia-northeast3-a`
- Artifact Registry: `asia-northeast3-docker.pkg.dev/git-ai-ops-practice/notiflex`
- kubectl context: `gke-sysnet4admin_book_gitaiops`
- Gateway 외부 IP: `35.216.118.49` (notiflex-gateway, HTTP:80)
- Proxy-only 서브넷: `proxy-only-subnet` (asia-northeast3, 172.16.0.0/23)

## 트러블슈팅 이력

독자가 겪은 문제와 해결 방법을 기록한다. 같은 문제를 다시 겪지 않도록 한다.

| 챕터 | 문제 | 해결 |
|------|------|------|
| 2.5 | GKE 생성 직후 `kubectl` 명령이 `gke-gcloud-auth-plugin ... not found`로 실패 | `gcloud components install gke-gcloud-auth-plugin` 으로 별도 설치. Homebrew cask google-cloud-sdk는 이 플러그인을 기본 포함하지 않음 |
| 2.6 | 최초 배포된 pod 2개가 같은 노드에 스케줄된 뒤 Spot VM preemption으로 모두 Error 상태가 됨 | GKE가 대체 노드에서 자동 재스케줄. Spot VM의 정상 동작이며, 프로덕션에서는 anti-affinity로 pod를 여러 노드에 분산시키는 것이 안전 |
| 3.2 | Fine-grained PAT 등록 후에도 Sync가 `authorization failed: Write access to repository not granted`로 실패 | 토큰의 Repository permissions에서 **Contents: Read-only**가 부여되지 않은 것이 원인. Repository access만 지정하고 Permissions를 건드리지 않으면 모든 권한이 No access. Contents: Read-only 부여한 새 토큰으로 Secret 교체 후 Sync 정상화 |
| 3.3 | `git push` 후에도 ArgoCD가 3분 주기 auto-sync 전까지 새 커밋을 감지하지 못함 | `kubectl annotate application <name> argocd.argoproj.io/refresh=hard --overwrite`로 즉시 refresh 트리거. 실습 중 대기 시간을 줄일 때 유용 |
| 4.3 | grafana/fluent-bit 차트가 deprecated + `image.repository` override가 무시되어 ImagePullBackOff | 가드레일의 공식 대안 `fluent/fluent-bit` 차트(fluent helm repo)로 전환. `kind: DaemonSet`, `config.inputs/filters/outputs`로 재구성 |
| 4.3 | Loki chart v6+에서 `useTestSchema`와 `schemaConfig`를 동시에 정의하면 `INSTALLATION FAILED` | 하나만 사용해야 함. schemaConfig v13을 명시하고 useTestSchema를 제거 |
| 4.3 | Loki chart가 SingleBinary와 SimpleScalable을 동시에 replicas>0으로 처리하려 해서 설치 실패 | `backend/read/write` replicas를 0으로 명시하여 SingleBinary만 사용하도록 강제 |
| 4.3 | Fluent Bit이 이미 존재하는 로그 파일의 이전 내용을 읽지 않아 notiflex-api 시작 로그(단발성)를 놓침 | `[INPUT] tail`에 `Read_from_Head On` 추가. 학습 환경에서는 기존 로그도 확인 필요 |
| 4.3 | Grafana port-forward가 반복적으로 끊김 → 원인은 Grafana 컨테이너 OOMKilled(exit 137). helm upgrade로 sidecar 컨테이너·datasource 추가되면서 메모리 사용량이 초기 튜닝값 256Mi를 초과 | `helm-values/kube-prometheus.yaml`의 `grafana.resources.limits.memory`를 256Mi → 512Mi로 상향 후 helm upgrade. ch6 진입 전 축소 시에도 grafana는 최소 384Mi 이상 유지 권장 |
| 4.3 | Spot VM 노드 1개(1js1) preemption으로 노드 1개만 남아 Grafana/Alertmanager/Prometheus Pending. GKE Autoscaler가 새 Spot 노드 프로비저닝하여 수 분 내 자동 복구 | Loki-0은 살아남아 로그 데이터 손실 없음. 반복되면 non-Spot 노드풀 병용 고려 |
| 5.3 | `kubectl apply -f install.yaml`이 `analysisruns`, `rollouts` CRD에서 `metadata.annotations: Too long: may not be more than 262144 bytes`로 실패. client-side apply가 last-applied-configuration annotation을 심는데 CRD 스키마가 초과 | `kubectl apply --server-side`로 재시도하면 통과. server-side apply는 field manager 방식이라 annotation 저장 안 함. 대형 CRD 설치의 사실상 표준 |
| 5.3 | Blue→Green promote 직후 외부 Gateway로 curl 시 순간적으로 `no healthy upstream` 2회 관찰 | active Service selector 전환 후 GKE GCLB 백엔드가 새 Pod IP를 healthy로 인지하기까지 헬스체크 사이클(checkIntervalSec=15s, unhealthyThreshold=2) 만큼 지연. 완전 zero-downtime을 원하면 HealthCheckPolicy checkIntervalSec를 5s로 낮추거나, prePromotionAnalysis로 백엔드 웜업 후 promote 필요. Notiflex 초기 단계에서는 수 초 히컵 허용 |
| 6.2 | Secret Manager CSI addon 활성화 후 노드 CPU 97/99%로 notiflex-api·valkey가 계속 Pending. GKE Managed Prometheus(`gmp-system`, `gke-managed-cim`)까지 함께 켜지면서 시스템 컴포넌트 CPU가 크게 증가 | (1) Loki + FluentBit 임시 uninstall (ch7.2 복원 예정), (2) notiflex-api / Valkey CPU requests 각각 10m로 재축소, (3) limits는 유지되어 실제 성능 영향 없음 |
| 6.2 | `helm upgrade valkey --set primary.resources.requests.cpu=10m` 이후에도 새로 만들어지는 valkey-primary-0이 여전히 CPU 50m을 요청. StatefulSet의 `currentRevision`이 갱신되지 않아 새 spec을 사용하지 않음 | `kubectl delete sts valkey-primary --cascade=orphan` 후 `helm upgrade` 재실행. PVC는 보존되어 데이터 유지, 새 StatefulSet이 10m spec으로 Pod 재생성 |
| 6.2 | Rollout Blue/Green 진행 중 이전 ReplicaSet(구 spec, 50m)의 Pod가 반복 재생성되며 노드 자원 점유, 신규 preview Pod가 valkey 연결 실패로 CrashLoopBackOff | `kubectl scale rs <old-rs>` 로 이전 ReplicaSet을 0으로 축소해 CPU 확보 → valkey 스케줄 성공 → preview Ready → 자동 promote 완료 |
| 6.2 | 앱이 scratch 베이스라 `kubectl exec` 시 `sh`/`ls` 실행 파일이 없어 CSI 마운트 파일을 직접 확인 불가 | Pod 로그의 `valkey password loaded from file: /mnt/secrets/valkey-password` 라인과 `kubectl get secretproviderclasspodstatuses`로 대체 검증. 필요 시 debug pod(busybox/alpine)를 별도로 띄워 마운트를 확인 |
