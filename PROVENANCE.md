# PROVENANCE.md — aflare 版权与来源声明 / Copyright & Provenance Declaration

> **状态 / Status: DRAFT — 未签名 / UNSIGNED**
>
> 本文件由项目所有者签署后方为有效。签署方式：所有者逐项确认第 2、3、4 节的待决问题，
> 删除本提示块，并以所有者 git 身份提交本文件（提交即签署）。
>
> This declaration takes effect only after the project owner signs it: confirm
> every open question in §2/§3/§4, remove this notice block, and commit this
> file with the owner's git identity (the commit IS the signature).

---

## 1. 目的 / Purpose

为商业授权尽调（dual-license due diligence）提供单一事实来源：本仓库的代码从何而来、
版权归谁、有无第三方权利残留。

Single source of truth for the dual-licensing program: where the code came
from, who holds copyright, and which third-party interests (if any) remain.

## 2. 身份声明 / Identity Declaration

历史提交中出现的作者身份及其归属。**待确认项必须由所有者逐个核实后改判。**

| Git 身份 | 提交数 | 归属 | 状态 |
|----------|--------|------|------|
| `alib8b8 <115916856+alib8b8@users.noreply.github.com>` | 124 | 所有者 / owner | ✅ 确认 |
| `alib8b8 <alib8b8@users.noreply.github.com>` | 109 | 所有者（同一 GitHub 账号，邮箱拼写不同）/ owner, alternate email spelling | ✅ 确认 |
| `alib8b8 <sjxj19921205@gmail.com>` | 0（历史）—— 2026-09 起的提交使用 / 0 (historical); used for commits from 2026-09 onward | 所有者 GitHub 已验证登记邮箱，规范身份（.mailmap 的 canonical）/ owner's GitHub-verified registration email, canonical identity (see .mailmap) | ✅ 确认 |
| `Dev <dev@example.com>` | 310 | **待确认** — 所有者的本地开发身份？ | ⬜ 未确认 |
| `Security Audit Bot <security@llm-box.local>` | 90 | **待确认** — 所有者运营的自动化机器人？ | ⬜ 未确认 |
| `HKAIC User <user@hkaic.example.com>` | 79 | **待确认** — 所有者在 HKAIC 平台上的会话身份？ | ⬜ 未确认 |
| `llm-box dev <dev@llm-box.local>` | 53 | **待确认** — 所有者在 llm-box 平台上的会话身份？ | ⬜ 未确认 |
| `llm-box <aln48xF1OWfWMFC8.llm-box@noreply.gitcode.com>` | 1 | **待确认** — 所有者经 GitCode 镜像推送？ | ⬜ 未确认 |
| `dependabot[bot]` | 40 | GitHub 机器人 / bot | ✅ 无需确认 |
| `github-actions` | 1 | CI 机器人 / bot | ✅ 无需确认 |
| `webbrain-one` | 1 | 第三方贡献者 / third-party contributor | 见 §3 / see §3 |

### ⚠️ 签名前必须回答的问题 / Questions to answer BEFORE signing

**Q1（雇佣/委托关系，最关键 / employment, the critical one）**
项目开发期间（llm-box → aflare 阶段），你与 HKAIC、llm-box 或任何雇主/委托方之间
是否存在雇佣、委托开发、或「职务作品」关系？若存在，相应期间产生的代码可能依法
归雇主或委托方所有，**必须先取得书面豁免或权利转让，再签本文件**。

During the llm-box → aflare development period, did you have any employment,
contract-for-hire, or work-made-for-hire relationship with HKAIC, llm-box, or
any other party? If yes, code from that period may legally belong to that
party — obtain a written waiver or assignment BEFORE signing.

**Q2（机器人归属）** `Security Audit Bot`（90 个提交）是否完全由你运营、其产出
无第三方权利主张？

**Q3（GitCode 推送）** `llm-box <...@noreply.gitcode.com>` 的 1 个提交是否为你本人经
GitCode 镜像推送？

## 3. 第三方贡献披露 / Third-Party Contributions

**webbrain-one**（GitHub 用户 295484252+webbrain-one）在提交 `c62e13f`
（"fix: update skill documentation to meet marketplace quality standards"）
中贡献了两个文档文件，合计 233 行：

| 文件 | 处置 / Disposition |
|------|--------------------|
| `.claude/skills/llm-box/SKILL.md`（122 行） | 已在 `5030b29`（llm-box → aflare 改名）中删除，无残留 / deleted in the rename commit, no residue |
| `.claude/skills/setup/SKILL.md`（111 行） | **已整体重写**（见下）/ **fully rewritten** (below) |

`.claude/skills/setup/SKILL.md` 于 2026-09-01 由项目所有者基于本仓库源码自行核实的
事实（CLI 命令面、安装脚本、工作流布局）重新撰写，不再保留原贡献者的任何表达。
重写后仓库不含 webbrain-one 的可版权表达。

The file was rewritten from facts verified against this repository's own
source (CLI command surface, install scripts, workspace layout); none of the
original contributor's expression survives. After the rewrite the tree
contains no copyrightable expression authored by webbrain-one.

注：仓库自 [Unreleased] 起的贡献条款（CONTRIBUTING.md §License）已要求新贡献授予
平行商业许可；本节披露的是条款生效**之前**的历史贡献。

## 4. 依赖许可 / Dependency Licensing

`go.mod` 直接依赖经核查均为宽松许可（MIT / Apache-2.0 / BSD 等），
**无 AGPL/GPL 传染性依赖**——商业版二进制分发不因依赖产生开源义务。

All direct Go dependencies are permissively licensed (MIT / Apache-2.0 / BSD);
no copyleft dependencies exist, so commercial binary distribution carries no
source-disclosure obligation from third-party code.

## 5. 联系渠道与签署人身份 / Contact Channels & Signatory Identity

所有者使用两个互不混用的邮箱，分工如下 / The owner maintains two
non-interchangeable mailboxes:

| 邮箱 / Email | 用途 / Role |
|--------------|-------------|
| `sjxj19921205@gmail.com` | **开发者身份邮箱**——GitHub 账号 alib8b8 的已验证登记邮箱、git 提交身份（§2 表中的规范身份，.mailmap 的 canonical）。签署本文件的提交将以该邮箱为作者 / **developer identity** — GitHub-verified registration email of alib8b8 and the git commit identity (the canonical identity in §2); the signing commit will be authored with this address |
| `local_first_agent@126.com` | **aflare 商业合作邮箱**——双许可询价、企业 CLA 签署件、商业合同往来（见 [LICENSE-COMMERCIAL.md](LICENSE-COMMERCIAL.md)） / **aflare business channel** — commercial-license inquiries, corporate CLA signatures, and contract correspondence (see [LICENSE-COMMERCIAL.md](LICENSE-COMMERCIAL.md)) |

两个邮箱均归所有者一人控制；商业主体与代码作者为同一人。
Both mailboxes are controlled by the same owner; the commercial party
and the code author are one and the same person.

## 6. 签署 / Signature

> （所有者确认 §2–§4 全部待决项后填写 / To be completed by the owner after
> resolving every open item in §2–§4）

```
签署人 / Signed:  alib8b8 <sjxj19921205@gmail.com>
GitHub:           https://github.com/alib8b8
日期 / Date:      YYYY-MM-DD
声明 / Statement: 本人确认本文件第 2、3、4 节所述内容真实、完整，
                  本人拥有 aflare 代码树的全部版权，有权授予 AGPL v3.0
                  社区许可及商业许可。
                  I confirm §2–§4 are true and complete, that I hold all
                  copyright in the aflare code tree, and that I am entitled
                  to grant both the AGPL v3.0 community license and
                  commercial licenses.
```
