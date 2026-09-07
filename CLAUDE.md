# Notiflex Platform

Notiflex — B2B 알림 SaaS 플랫폼의 인프라 및 애플리케이션 저장소.

## 프로젝트 개요

- **서비스**: 다양한 채널(웹훅, 이메일, 슬랙 등)로 알림을 전송하는 B2B SaaS
- **책 참조**: 「AI 시대에 개발자가 알아야 하는 인프라 구성 배포 with 클로드 코드」 실습 저장소
- **가이드 저장소 (형제 디렉터리)**: `../\_Book_GitAIOps`

## 기술 스택

- **언어**: Go (표준 라이브러리, 외부 프레임워크 없음)
- **컨테이너**: multi-stage 빌드 + `scratch` 베이스 이미지
- **오케스트레이션**: GKE Standard (Zonal, Spot VM)
- **GitOps**: ArgoCD (3장에서 도입)
- **CI**: GitHub Actions (3장에서 도입, SHA 태그 자동 커밋으로 ArgoCD 연동)
- **관측 가능성**: Prometheus, Grafana, Loki, Fluent Bit, Tempo (4·8장)
- **외부 트래픽**: GKE Gateway API + HealthCheckPolicy (5장에서 도입)
- **배포 컨트롤러**: Argo Rollouts (5장에서 도입, Blue/Green → 6장 Canary 전환 예정)
- **배포 전략**: Rolling → Blue/Green → Canary (점진 진화)

## GCP 설정

| 항목 | 값 |
|------|-----|
| Project ID | `git-ai-ops-practice` |
| Region | `asia-northeast3` (서울) |
| Zone | `asia-northeast3-a` |
| Artifact Registry | `asia-northeast3-docker.pkg.dev/git-ai-ops-practice/notiflex` |

## 디렉터리 구조

```
notiflex-platform/
├── CLAUDE.md
├── JOURNEY.md            # 실제 진행 이력·도구 선택·현재 버전·트러블슈팅 (AI가 각 챕터 완료 시 갱신)
├── app/                  # Go 애플리케이션 소스
├── argocd/               # ArgoCD Application 매니페스트
├── docs/
│   └── architecture-decisions.md  # ADR (5장에서 도입)
├── helm-values/          # Helm values 파일 (kube-prometheus, loki, fluent-bit)
├── k8s/
│   └── smb/              # Kubernetes 매니페스트 (SMB = Single-cluster, Manifest-based, Basic)
│                         #   Gateway, HTTPRoute, HealthCheckPolicy, Rollout, Service(active+preview)
└── .github/
    └── workflows/        # GitHub Actions CI 파이프라인
```

## 행동 규칙 (Claude Code용)

1. **항상 현재 상태 먼저 확인.** 파일 수정 전 `Read`, 클러스터 변경 전 `kubectl --context gke-sysnet4admin_book_gitaiops get ...`으로 현재 상태를 확인한 뒤 진행한다.
2. **kubectl 안전 규칙.** 이 저장소의 kubectl 명령은 반드시 `--context gke-sysnet4admin_book_gitaiops`를 붙인다. 잘못된 클러스터에 명령이 나가지 않도록 한다.
3. **파괴적 명령은 승인 후 실행.** `kubectl delete`, `gcloud ... delete`, `git push --force` 같은 되돌리기 어려운 명령은 무엇을·왜 지우는지 먼저 설명한다.
4. **Artifact Registry 태그.** 이미지 태그는 `<git-short-sha>` 또는 `<semver>`를 사용한다. `latest`는 사용하지 않는다.
5. **커밋 메시지.** Conventional Commits 스타일 (`feat:`, `fix:`, `docs:`, `chore:` 등)을 사용한다.
