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
| ch3 | 3.3 기능 추가 | ⬜ | | |
| ch3 | 3.4 CI | ⬜ | | |
| ch3 | 3.5 CI-CD 연결 | ⬜ | | |
| ch4 | 4.2 메트릭 모니터링 | ⬜ | | |
| ch4 | 4.3 로그 수집 | ⬜ | | |
| ch4 | 4.4 알림 | ⬜ | | |
| ch5 | 5.2 트래픽 관리 | ⬜ | | |
| ch5 | 5.3 무중단 배포 | ⬜ | | |
| ch6 | 6.1 캐시 | ⬜ | | |
| ch6 | 6.2 시크릿 관리 | ⬜ | | |
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

## 현재 버전

| 컴포넌트 | 버전 | 변경 이력 |
|---------|------|----------|
| Go | 1.25 | 2026-09-03 초기 설정 (ch6 valkey-go, ch8 OTel SDK 대비) |
| Notiflex 이미지 | v0.1.0 | 2026-09-03 초기 빌드 (Cloud Build, digest `sha256:b587b653...`) |
| ArgoCD | v3.5.2 | 2026-09-04 설치 (stable manifest) |
| Kafka | | |
| OTel SDK | | |

## 현재 리소스

| 노드풀 | 머신 타입 | 노드 수 | 주요 워크로드 |
|--------|----------|---------|-------------|
| default-pool | e2-medium (Spot, disk 30GB) | 2 | notiflex-api (2 replicas) |

**GCP 컨텍스트**
- Project: `git-ai-ops-practice`
- Region/Zone: `asia-northeast3` / `asia-northeast3-a`
- Artifact Registry: `asia-northeast3-docker.pkg.dev/git-ai-ops-practice/notiflex`
- kubectl context: `gke-sysnet4admin_book_gitaiops`

## 트러블슈팅 이력

독자가 겪은 문제와 해결 방법을 기록한다. 같은 문제를 다시 겪지 않도록 한다.

| 챕터 | 문제 | 해결 |
|------|------|------|
| 2.5 | GKE 생성 직후 `kubectl` 명령이 `gke-gcloud-auth-plugin ... not found`로 실패 | `gcloud components install gke-gcloud-auth-plugin` 으로 별도 설치. Homebrew cask google-cloud-sdk는 이 플러그인을 기본 포함하지 않음 |
| 2.6 | 최초 배포된 pod 2개가 같은 노드에 스케줄된 뒤 Spot VM preemption으로 모두 Error 상태가 됨 | GKE가 대체 노드에서 자동 재스케줄. Spot VM의 정상 동작이며, 프로덕션에서는 anti-affinity로 pod를 여러 노드에 분산시키는 것이 안전 |
| 3.2 | Fine-grained PAT 등록 후에도 Sync가 `authorization failed: Write access to repository not granted`로 실패 | 토큰의 Repository permissions에서 **Contents: Read-only**가 부여되지 않은 것이 원인. Repository access만 지정하고 Permissions를 건드리지 않으면 모든 권한이 No access. Contents: Read-only 부여한 새 토큰으로 Secret 교체 후 Sync 정상화 |
