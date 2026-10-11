# GitHub Branch Protection 配置 (master)

> 本文档**不**是自动化的, 是**人工**去 GitHub 网页操作清单.  
> 操作一次后, 截图存档, 后续 reviewer 凭此核对.  
> 流程参考: [`docs/process/new-access-layer.md`](../process/new-access-layer.md) §5.3

---

## 1. 入口

仓库 → **Settings** → **Branches** → **Branch protection rules** → **Add rule** / **Edit** (master)

---

## 2. 必勾项

### 2.1 Branch name pattern

- `master`

### 2.2 Protect matching branches

- [x] **Require a pull request before merging**
  - [x] **Require approvals**: `1`
  - [x] **Dismiss stale pull request approvals when new commits are pushed**
  - [x] **Require review from Code Owners** (= `CODEOWNERS` 生效, 所有路径 = @zhangjun005)
- [x] **Require status checks to pass before merging**
  - [x] **Require branches to be up to date before merging**
  - 必选 status checks (见 §3):
    - [ ] `go vet ./...`
    - [ ] `go test ./...`
    - [ ] `gofmt -l .` (或 `gofmt -d .` 无 diff)
- [x] **Require conversation resolution before merging**
- [x] **Require linear history** (rebase / squash only, **不**允许 merge commit)
- [x] **Require signed commits** (强烈建议, 防密钥泄露后 commit 伪造)
- [x] **Include administrators** ✅ (管理员也守规则)

### 2.3 **不**勾项 (明确不勾, 防止歧义)

- [ ] ❌ **Allow force pushes** — `force-push` 在 `pending-decisions.md` §S5 中**禁止**
- [ ] ❌ **Allow deletions** — master 永远**不**删
- [ ] ❌ **Allow specified actors to bypass pull request requirements** — **没有** 例外, 包括 zhangjun005 本人

---

## 3. 必设的 status checks

需**先**在 `.github/workflows/ci.yml` (待写) 暴露这 3 个 check:

```yaml
# .github/workflows/ci.yml (待写, 本 PR 不在本批范围)
name: ci
on: [push, pull_request]
jobs:
  test:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with: { go-version: '1.27' }
      - run: go vet ./...
      - run: go test ./...
      - run: |
          output=$(gofmt -l .)
          if [ -n "$output" ]; then echo "gofmt diff:"; echo "$output"; exit 1; fi
```

**当前** (2026-10-07) CI 状态:
- ❌ `.github/workflows/ci.yml` **未**建
- ⚠️ `go vet` / `go test` / `gofmt` 必**手动**在本地跑 (AGENTS.md §"Workflow" 第 5 步已写)
- ⚠️ branch protection 配好后, status check **找不到** → 暂时只勾 "Require status checks to pass" 但**不**勾具体 check 名, 等 CI workflow 落地**再**补

---

## 4. 操作后核对

配完**后**, 用**未授权**的 GitHub 账号 (或 incognito 窗口) 试:
- [ ] 直接 push 到 master → 应**失败** "protected branch"
- [ ] 提 PR 但**无** approval → 应**失败** "Review required"
- [ ] PR 改 `docs/abstractDesign/architecture.md` → CODEOWNERS 应**要求** @zhangjun005 review
- [ ] PR 改 `frontend/` 目录 → CODEOWNERS 应**要求** @zhangjun005 review (v1.3+ 启用**后**)

---

## 5. 与 `CODEOWNERS` 联动

`CODEOWNERS` 强制**每**个 PR 都**至少**一个 owner approval. 配置后**所有**改动都受 §2.2 约束.

`CODEOWNERS` 路径注释掉的段 (`# /cmd/ ...` 等) 在**当前**没文件, 等 v1.3+ **实际**出现 `cmd/kron/serve-lsp/` 时**取消**注释.

---

## 6. 待办 (后续 chat 写)

- [ ] `.github/workflows/ci.yml` (v1.3 之前**必须**建, 否则 §3 必设的 status check 找不到)
- [ ] **强制** signed commits (本地 git config `commit.gpgsign true`)
- [ ] `MAINTAINERS.md` 仓库根 (本批**不**建, 暂以 AGENTS.md "作者" 段替代)
