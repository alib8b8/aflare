// Copyright (c) 2026 aflare Contributors
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published
// by the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <https://www.gnu.org/licenses/>.

package core

// --- Outbound data monitoring ---------------------------------------------
//
// OutboundDataMonitor（滑动窗口出站流量监控器）的具体实现位于 nodes 包。
// core 与 providers 无法 import nodes（nodes 已 import core，会形成循环依赖），
// 因此这里定义一个最小接口 OutboundRecorder，由 nodes 在 init 时通过
// SetGlobalOutboundRecorder 把它的 singleton 注入进来；core 与 providers 通过
// RecordOutbound 上报出站字节数，使 README 宣称的"出站数据量异常监控"真正生效。

// OutboundRecorder 记录一次出站发送的字节数，必要时触发异常告警。
type OutboundRecorder interface {
	Record(bytes int)
}

// globalOutboundRecorder 是进程级出站数据监控器，由 nodes 包在 init 时注入。
// 为 nil 表示监控已关闭（AFLARE_OUTBOUND_MONITOR_DISABLE=1）或 nodes 尚未初始化。
// 只在 Execute 调用时读取；RecordOutbound 的 nil 检查保证安全。
var globalOutboundRecorder OutboundRecorder

// SetGlobalOutboundRecorder 安装进程级出站数据监控器，供 nodes 包在 init 时
// 调用一次。传 nil 即关闭监控。
func SetGlobalOutboundRecorder(r OutboundRecorder) {
	globalOutboundRecorder = r
}

// GlobalOutboundRecorder 返回已安装的出站监控器，未安装则返回 nil。
func GlobalOutboundRecorder() OutboundRecorder {
	return globalOutboundRecorder
}

// RecordOutbound 向全局监控器上报 n 字节出站数据（best-effort）。
// 监控器为 nil 或 n 非正时为 no-op；OutboundDataMonitor.Record 自身已对回调
// panic 做 recover，因此本调用绝不影响主流程。
func RecordOutbound(n int) {
	if r := globalOutboundRecorder; r != nil {
		r.Record(n)
	}
}
