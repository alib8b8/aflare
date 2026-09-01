# aflare Licensing Guide — 哪个协议适合你 / Which License Applies to You

aflare 采用双许可（dual licensing）：同一份代码，两条授权路径——
**AGPL v3.0 社区版**（免费）或**商业授权**（付费）。本页帮你判断该走哪条。

aflare ships under a dual license: the free **AGPL v3.0 community edition**
or a paid **commercial license**. This page tells you which one you need.

---

## 决策表 / Decision Table

| 你的场景 / Your scenario | 适用协议 / License |
|--------------------------|--------------------|
| 个人学习、本地运行、改着玩 / personal use, local runs, tinkering | AGPL v3.0 |
| 内部使用，且整个组合作品可以 AGPL 开源 / internal use where the combined work can be AGPL | AGPL v3.0 |
| 把 aflare（原样或修改后）嵌入闭源产品分发 / embedding in a closed-source product you distribute | **商业授权 / Commercial** |
| 基于 aflare 对外提供 SaaS 或内部平台服务，且不想开源调用方代码 / offering aflare-based SaaS without opening your surrounding code | **商业授权 / Commercial** |
| 把 aflare 作为 Go 库 import 进自己的闭源服务 / importing aflare packages into a closed-source Go service | **商业授权 / Commercial** |
| 项目本身已是 AGPL v3.0 兼容的开源项目 / your project is already AGPL-compatible OSS | AGPL v3.0 |

拿不准就问：[local_first_agent@126.com](mailto:local_first_agent@126.com)。
When in doubt, ask.

## AGPL 的两条关键义务 / The Two Obligations That Bite

判断是否需要商业版，看这两条你能否接受：

1. **§13 网络条款（AGPL 独有）**：用户**通过网络**使用你的软件（不必分发二进制），
   你就必须向这些用户提供整个组合作品的源码。把 aflare 包进你的服务端 = 你的服务端
   代码要 AGPL 开源。
2. **修改即传染**：对 aflare 本身的任何修改，分发或网络提供时须以 AGPL 开源。

## 为什么 Go 静态链接堵死了「弱链接」口子 / Why Go Static Linking Closes the Loophole

LGPL 时代有经典的规避话术：「我只动态链接，我的代码不受传染」。**Go 没有这个空间**：
Go 程序默认静态编译，import aflare 的包就是把 aflare 代码编进你的二进制——
你的程序是 aflare 的派生作品（derivative work），AGPL 对整个二进制生效。
想闭源，只能走商业授权；没有灰色地带。这对合规是好事：不需要律师辩论
「链接算不算衍生」，答案永远是「算」。

Go binaries statically link by default — importing aflare packages makes your
program an unambiguous derivative work, so AGPL applies to the whole binary.
There is no dynamic-linking dodge, no grey area to argue about.

## FAQ

**社区版是试用版吗？/ Is the community edition a trial?**
不是。AGPL v3.0 是完整功能、永久免费的许可。商业版只为无法履行 AGPL 义务的场景存在。
No. AGPL v3.0 is full-featured and permanent. The commercial license exists
solely for those who cannot comply with AGPL.

**我只在 CI 里跑 aflare 工作流，没改代码，需要商业版吗？/ I just run aflare in CI, unmodified.**
不需要——你是 AGPL 意义上的「用户」，使用本身不触发义务；分发或网络服务才触发。
No. Merely running the software triggers no AGPL obligation; distribution
and network offering do.

**我可以 fork 吗？/ Can I fork?**
可以，按 AGPL 条款。注意 "aflare" 是项目名，fork 请换品牌。
Yes, under AGPL terms. Use your own branding for forks.

**商业授权包含什么？/ What does the commercial license include?**
永久、全球、免版税的使用/修改/分发权利，豁免 AGPL 开源义务；按约定范围的组织级授权；
优先支持。详见 [LICENSE-COMMERCIAL.md](../LICENSE-COMMERCIAL.md)。
Perpetual, royalty-free use/modification/distribution without AGPL
disclosure duties; org-wide scope; priority support. See
[LICENSE-COMMERCIAL.md](../LICENSE-COMMERCIAL.md).

**怎么买？/ How do I buy?**
邮件 [local_first_agent@126.com](mailto:local_first_agent@126.com)，附公司名、
使用场景、大致规模，通常两个工作日内回复。
Email with company name, use case, and approximate scale; we usually reply
within two business days.

**贡献的代码用什么协议？/ What license do contributions use?**
AGPL v3.0 出站 + 授予项目所有者平行商业再许可权（版权保留，非转让），
见 [CONTRIBUTING.md](../CONTRIBUTING.md)。
AGPL v3.0 outbound plus a parallel commercial relicensing grant to the
project owner — see [CONTRIBUTING.md](../CONTRIBUTING.md).

## 相关文件 / Related Files

- [LICENSE](../LICENSE) — AGPL v3.0 全文 / full AGPL v3.0 text
- [LICENSE-COMMERCIAL.md](../LICENSE-COMMERCIAL.md) — 商业授权说明 / commercial licensing overview
- [PROVENANCE.md](../PROVENANCE.md) — 版权与来源声明 / copyright & provenance declaration
