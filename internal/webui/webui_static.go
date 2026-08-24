// Copyright (c) 2026 aflare Contributors
//
// aflare‍​‌​​​​​‌​‌​​​‌‌​​‌​​‌‌​​​‌​‌​​‌​​​​​​​‌​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​‌‌​‌​‌​‌​​​‌‌​​​​‌‌‌​​‌​​‌​‌​‌​‌‌‌​​‌​​‌‌‌​​‌‌‌‌​‌‌​​​‌​‌​​‌​‌​​‌‌‌‌​​​‌‌​‌​‌​​​​​​​​​​​​​​​​​​‌​​‌‌​​​‌​‌‌​​‌‌⁠
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

package webui

import (
	"net/http"
)

func (s *WebUIServer) handleIndex(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(indexHTML))
}

var indexHTML = `<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Aflare - Workflow Visualizer</title>
    <style>
        * { margin: 0; padding: 0; box-sizing: border-box; }
        body { font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif; background: #0f0f23; color: #e0e0e0; }
        .container { display: flex; height: 100vh; }
        .sidebar { width: 320px; background: #1a1a2e; border-right: 1px solid #2a2a4a; display: flex; flex-direction: column; }
        .sidebar-header { padding: 16px; border-bottom: 1px solid #2a2a4a; }
        .sidebar-header h1 { font-size: 18px; font-weight: 600; color: #00d4ff; }
        .sidebar-content { flex: 1; overflow-y: auto; padding: 12px; }
        .sidebar-footer { padding: 12px; border-top: 1px solid #2a2a4a; }
        .btn { display: block; width: 100%; padding: 10px 12px; border: none; border-radius: 8px; cursor: pointer; font-size: 14px; font-weight: 500; transition: all 0.2s; }
        .btn-primary { background: #00d4ff; color: #0f0f23; }
        .btn-primary:hover { background: #00b8e6; }
        .btn-secondary { background: #2a2a4a; color: #e0e0e0; }
        .btn-secondary:hover { background: #3a3a5a; }
        .btn-danger { background: #ff4757; color: white; }
        .btn-danger:hover { background: #ff3344; }
        .workflow-list { list-style: none; }
        .workflow-item { padding: 10px; margin-bottom: 6px; background: #2a2a4a; border-radius: 8px; cursor: pointer; transition: all 0.2s; }
        .workflow-item:hover { background: #3a3a5a; }
        .workflow-item.active { background: #00d4ff; color: #0f0f23; }
        .main { flex: 1; display: flex; flex-direction: column; }
        .toolbar { padding: 12px 16px; background: #1a1a2e; border-bottom: 1px solid #2a2a4a; display: flex; gap: 12px; align-items: center; }
        .toolbar select, .toolbar input { padding: 8px 12px; background: #2a2a4a; border: 1px solid #3a3a5a; border-radius: 6px; color: #e0e0e0; font-size: 14px; }
        .toolbar select:focus, .toolbar input:focus { outline: none; border-color: #00d4ff; }
        .tabs { display: flex; border-bottom: 1px solid #2a2a4a; }
        .tab { padding: 10px 20px; cursor: pointer; font-size: 14px; font-weight: 500; color: #8080a0; border-bottom: 2px solid transparent; transition: all 0.2s; }
        .tab.active { color: #00d4ff; border-bottom-color: #00d4ff; }
        .tab-content { flex: 1; overflow: auto; padding: 16px; }
        textarea { width: 100%; height: 100%; min-height: 500px; padding: 16px; background: #0f0f23; border: 1px solid #2a2a4a; border-radius: 8px; color: #e0e0e0; font-family: 'Monaco', 'Menlo', monospace; font-size: 14px; resize: none; }
        textarea:focus { outline: none; border-color: #00d4ff; }
        .preview { background: #1a1a2e; border-radius: 8px; padding: 16px; font-family: 'Monaco', 'Menlo', monospace; font-size: 13px; white-space: pre-wrap; word-break: break-all; max-height: 100%; overflow: auto; }
        .mermaid-container { background: #1a1a2e; border-radius: 8px; padding: 16px; overflow: auto; }
        .visualization-svg { width: 100%; height: auto; }
        .status-bar { padding: 8px 16px; background: #1a1a2e; border-top: 1px solid #2a2a4a; font-size: 12px; color: #8080a0; display: flex; gap: 20px; }
        .status-bar .valid { color: #00ff88; }
        .status-bar .invalid { color: #ff4757; }
        .modal { display: none; position: fixed; top: 0; left: 0; right: 0; bottom: 0; background: rgba(0,0,0,0.8); justify-content: center; align-items: center; z-index: 1000; }
        .modal.show { display: flex; }
        .modal-content { background: #1a1a2e; border: 1px solid #2a2a4a; border-radius: 12px; padding: 24px; width: 400px; }
        .modal-content h2 { margin-bottom: 16px; font-size: 18px; }
        .modal-content input { width: 100%; padding: 10px; margin-bottom: 16px; background: #2a2a4a; border: 1px solid #3a3a5a; border-radius: 6px; color: #e0e0e0; }
        .modal-content input:focus { outline: none; border-color: #00d4ff; }
        .modal-actions { display: flex; gap: 12px; justify-content: flex-end; }
        .error-message { color: #ff4757; font-size: 12px; margin-top: 8px; }
        .warnings { background: #2a2a0a; border: 1px solid #4a4a1a; border-radius: 6px; padding: 10px; margin-top: 10px; font-size: 12px; color: #ffff80; }
        .chat-container { flex: 1; display: flex; flex-direction: column; height: 100%; }
        .chat-messages { flex: 1; overflow-y: auto; padding: 16px; display: flex; flex-direction: column; gap: 12px; }
        .chat-message { max-width: 85%; padding: 12px 16px; border-radius: 12px; font-size: 14px; line-height: 1.5; }
        .chat-message.user { align-self: flex-end; background: #00d4ff; color: #0f0f23; }
        .chat-message.assistant { align-self: flex-start; background: #2a2a4a; color: #e0e0e0; }
        .chat-message.error { align-self: flex-start; background: #4a1a1a; color: #ff4757; }
        .chat-input-area { padding: 12px 16px; border-top: 1px solid #2a2a4a; display: flex; gap: 10px; }
        .chat-input-area input { flex: 1; padding: 10px 14px; background: #2a2a4a; border: 1px solid #3a3a5a; border-radius: 8px; color: #e0e0e0; font-size: 14px; }
        .chat-input-area input:focus { outline: none; border-color: #00d4ff; }
        .chat-input-area button { padding: 10px 20px; }
        .chat-loading { align-self: flex-start; color: #8080a0; font-size: 13px; padding: 8px 16px; }
    </style>
</head>
<body>
    <div class="container">
        <div class="sidebar">
            <div class="sidebar-header">
                <h1>Aflare</h1>
            </div>
            <div class="sidebar-content">
                <ul class="workflow-list" id="workflowList"></ul>
            </div>
            <div class="sidebar-footer">
                <button class="btn btn-primary" onclick="showNewModal()">+ New Workflow</button>
            </div>
        </div>

        <div class="main">
            <div class="toolbar">
                <select id="outputFormat" onchange="renderVisualization()">
                    <option value="mermaid">Mermaid</option>
                    <option value="json">JSON</option>
                    <option value="dot">DOT</option>
                    <option value="ascii">ASCII</option>
                </select>
                <button class="btn btn-secondary" onclick="validateWorkflow()">Validate</button>
                <button class="btn btn-primary" onclick="saveWorkflow()">Save</button>
                <button class="btn btn-danger" onclick="deleteCurrentWorkflow()" style="display:none" id="deleteBtn">Delete</button>
                <input type="text" id="workflowName" placeholder="Workflow name..." />
            </div>

            <div class="tabs">
                <div class="tab active" onclick="switchTab('editor')">Editor</div>
                <div class="tab" onclick="switchTab('preview')">Preview</div>
                <div class="tab" onclick="switchTab('chat')">Chat</div>
            </div>

            <div class="tab-content" id="editorTab">
                <textarea id="workflowEditor" placeholder="Enter workflow YAML here..."></textarea>
            </div>

            <div class="tab-content" id="previewTab" style="display:none">
                <div id="previewContent"></div>
            </div>

            <div class="tab-content" id="chatTab" style="display:none">
                <div class="chat-container">
                    <div class="chat-messages" id="chatMessages">
                        <div class="chat-message assistant">Hello! I'm aflare, your local-first automation agent. Ask me anything — I can run workflows, search templates, compose new automations, and more.</div>
                    </div>
                    <div class="chat-input-area">
                        <input type="text" id="chatInput" placeholder="Type your message..." onkeydown="if(event.key==='Enter')sendChat()" />
                        <button class="btn btn-primary" onclick="sendChat()">Send</button>
                        <button class="btn btn-secondary" onclick="clearChat()" title="Clear conversation">Clear</button>
                    </div>
                </div>
            </div>

            <div class="status-bar">
                <span id="validationStatus">Not validated</span>
                <span id="stepCount">0 steps</span>
            </div>
        </div>
    </div>

    <div class="modal" id="newModal">
        <div class="modal-content">
            <h2>New Workflow</h2>
            <input type="text" id="newWorkflowName" placeholder="Workflow name" />
            <div class="error-message" id="newError"></div>
            <div class="modal-actions">
                <button class="btn btn-secondary" onclick="hideNewModal()">Cancel</button>
                <button class="btn btn-primary" onclick="createNewWorkflow()">Create</button>
            </div>
        </div>
    </div>

    <script>
        // Mermaid is loaded from a public CDN. In air-gapped/intranet
        // environments the CDN is unreachable, so we detect load failure
        // and fall back to rendering the Mermaid source as preformatted
        // text (see renderVisualization) instead of leaving a blank page.
        // The server-side visualizer already generates the Mermaid source,
        // so the diagram structure stays visible/copyable offline; only the
        // rendered graph degrades. Vendoring the ~3MB mermaid.min.js into
        // the binary was considered but rejected to keep the binary lean.
        window.mermaidAvailable = false;
    </script>
    <script src="https://cdn.jsdelivr.net/npm/mermaid@10/dist/mermaid.min.js"
            onload="window.mermaidAvailable=true"
            onerror="console.warn('Mermaid CDN unreachable (offline/intranet mode): showing diagram source as text.')"></script>
    <script>
        let currentWorkflow = '';

        // Generate a persistent session ID for this browser tab.
        function getSessionId() {
            let id = localStorage.getItem('aflare_session_id');
            if (!id) {
                id = crypto.randomUUID ? crypto.randomUUID() : 'sess-' + Date.now().toString(36) + Math.random().toString(36).slice(2, 8);
                localStorage.setItem('aflare_session_id', id);
            }
            return id;
        }

        async function loadWorkflows() {
            try {
                const response = await fetch('/api/workflows');
                const data = await response.json();
                const list = document.getElementById('workflowList');
                list.innerHTML = '';
                data.workflows.forEach(name => {
                    const li = document.createElement('li');
                    li.className = 'workflow-item';
                    li.textContent = name;
                    li.onclick = () => loadWorkflow(name);
                    list.appendChild(li);
                });
            } catch (e) {
                console.error('Failed to load workflows:', e);
            }
        }

        async function loadWorkflow(name) {
            try {
                const response = await fetch('/api/workflow?name=' + encodeURIComponent(name));
                if (response.ok) {
                    const content = await response.text();
                    document.getElementById('workflowEditor').value = content;
                    document.getElementById('workflowName').value = name;
                    currentWorkflow = name;
                    document.getElementById('deleteBtn').style.display = 'block';

                    document.querySelectorAll('.workflow-item').forEach(item => {
                        item.classList.remove('active');
                        if (item.textContent === name) item.classList.add('active');
                    });

                    renderVisualization();
                }
            } catch (e) {
                console.error('Failed to load workflow:', e);
            }
        }

        async function saveWorkflow() {
            const name = document.getElementById('workflowName').value.trim();
            const content = document.getElementById('workflowEditor').value;

            if (!name) {
                alert('Please enter a workflow name');
                return;
            }

            try {
                const response = await fetch('/api/workflow', {
                    method: 'POST',
                    headers: { 'Content-Type': 'application/json' },
                    body: JSON.stringify({ name, content })
                });

                if (response.ok) {
                    currentWorkflow = name;
                    await loadWorkflows();
                    document.querySelectorAll('.workflow-item').forEach(item => {
                        if (item.textContent === name) item.classList.add('active');
                    });
                    document.getElementById('deleteBtn').style.display = 'block';
                    alert('Workflow saved successfully');
                } else {
                    const data = await response.json();
                    alert('Failed to save: ' + (data.error || 'Unknown error'));
                }
            } catch (e) {
                console.error('Failed to save workflow:', e);
            }
        }

        async function deleteCurrentWorkflow() {
            if (!currentWorkflow) return;
            if (!confirm('Are you sure you want to delete this workflow?')) return;

            try {
                const response = await fetch('/api/workflow?name=' + encodeURIComponent(currentWorkflow), {
                    method: 'DELETE'
                });

                if (response.ok) {
                    document.getElementById('workflowEditor').value = '';
                    document.getElementById('workflowName').value = '';
                    document.getElementById('deleteBtn').style.display = 'none';
                    currentWorkflow = '';
                    document.querySelectorAll('.workflow-item').forEach(item => item.classList.remove('active'));
                    await loadWorkflows();
                }
            } catch (e) {
                console.error('Failed to delete workflow:', e);
            }
        }

        async function validateWorkflow() {
            const content = document.getElementById('workflowEditor').value;
            try {
                const response = await fetch('/api/validate', {
                    method: 'POST',
                    headers: { 'Content-Type': 'application/json' },
                    body: JSON.stringify({ workflow: content })
                });

                const data = await response.json();
                const status = document.getElementById('validationStatus');
                const steps = document.getElementById('stepCount');

                if (data.valid) {
                    status.textContent = '✓ Valid';
                    status.className = 'valid';
                    steps.textContent = data.steps + ' steps';
                } else {
                    status.textContent = '✗ Invalid: ' + data.error;
                    status.className = 'invalid';
                    steps.textContent = '0 steps';
                }
            } catch (e) {
                console.error('Failed to validate:', e);
            }
        }

        async function renderVisualization() {
            const content = document.getElementById('workflowEditor').value;
            const format = document.getElementById('outputFormat').value;
            const preview = document.getElementById('previewContent');

            try {
                const response = await fetch('/api/visualize?format=' + format, {
                    method: 'POST',
                    headers: { 'Content-Type': 'application/json' },
                    body: JSON.stringify({ workflow: content })
                });

                const result = await response.text();

                if (format === 'mermaid') {
                    if (window.mermaidAvailable && typeof mermaid !== 'undefined') {
                        preview.innerHTML = '<div class="mermaid-container"><div class="mermaid"></div></div>';
                        preview.querySelector('.mermaid').textContent = result;
                        try { mermaid.init(undefined, '.mermaid'); }
                        catch (e) {
                            // mermaid.init can throw on malformed graphs; show source.
                            preview.innerHTML = '<div class="preview">' + escapeHtml(result) + '</div>';
                        }
                    } else {
                        // Offline/intranet: CDN unreachable. Show the Mermaid
                        // source as preformatted text so the workflow graph
                        // structure stays readable/copyable without the renderer.
                        preview.innerHTML =
                            '<div class="mermaid-container">' +
                            '<div style="color:#8080a0;font-size:12px;margin-bottom:8px">' +
                            'Mermaid 渲染器离线不可用（CDN 不可达），以下为 Mermaid 源码，可复制到本地渲染器查看。' +
                            '</div>' +
                            '<div class="preview">' + escapeHtml(result) + '</div>' +
                            '</div>';
                    }
                } else if (format === 'json') {
                    preview.innerHTML = '<div class="preview">' + syntaxHighlight(result) + '</div>';
                } else {
                    preview.innerHTML = '<div class="preview">' + escapeHtml(result) + '</div>';
                }
            } catch (e) {
                preview.innerHTML = '<div class="preview">Error: ' + escapeHtml(e.message) + '</div>';
            }
        }

        function switchTab(tab) {
            document.querySelectorAll('.tab').forEach(t => t.classList.remove('active'));
            document.querySelectorAll('.tab-content').forEach(c => c.style.display = 'none');

            event.target.classList.add('active');
            document.getElementById(tab + 'Tab').style.display = 'block';

            if (tab === 'preview') {
                renderVisualization();
            }
        }

        function showNewModal() {
            document.getElementById('newModal').classList.add('show');
            document.getElementById('newWorkflowName').value = '';
            document.getElementById('newError').textContent = '';
        }

        function hideNewModal() {
            document.getElementById('newModal').classList.remove('show');
        }

        async function createNewWorkflow() {
            const name = document.getElementById('newWorkflowName').value.trim();
            if (!name) {
                document.getElementById('newError').textContent = 'Please enter a workflow name';
                return;
            }

            document.getElementById('workflowEditor').value = '# New Workflow\nname: ' + name + '\ndescription: \nsteps:\n';
            document.getElementById('workflowName').value = name;
            currentWorkflow = name;
            hideNewModal();
            document.getElementById('deleteBtn').style.display = 'none';
        }

        function escapeHtml(text) {
            return text.replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;').replace(/"/g, '&quot;');
        }

        function syntaxHighlight(json) {
            json = json.replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;');
            return json.replace(/("(\\u[a-zA-Z0-9]{4}|\\[^u]|[^\\"])*"(\s*:)?|\b(true|false|null)\b|-?\d+(?:\.\d*)?(?:[eE][+\-]?\d+)?)/g, function (match) {
                let cls = 'number';
                if (/^"/.test(match)) {
                    if (/:$/.test(match)) cls = 'key';
                    else cls = 'string';
                } else if (/true|false/.test(match)) cls = 'boolean';
                else if (/null/.test(match)) cls = 'null';
                return '<span style="color:' + (cls === 'key' ? '#00d4ff' : cls === 'string' ? '#00ff88' : cls === 'number' ? '#ffaa00' : cls === 'boolean' ? '#ff4757' : '#888') + '">' + match + '</span>';
            });
        }

        document.getElementById('workflowEditor').addEventListener('input', () => {
            document.getElementById('validationStatus').textContent = 'Not validated';
            document.getElementById('validationStatus').className = '';
        });

        // Chat functions
        async function sendChat() {
            const input = document.getElementById('chatInput');
            const message = input.value.trim();
            if (!message) return;

            const messages = document.getElementById('chatMessages');
            messages.innerHTML += '<div class="chat-message user">' + escapeHtml(message) + '</div>';
            input.value = '';
            input.disabled = true; // Disable input during streaming

            // Create assistant message element that we'll stream into
            const assistantDiv = document.createElement('div');
            assistantDiv.className = 'chat-message assistant';
            assistantDiv.innerHTML = '<span class="chat-loading">Thinking...</span>';
            messages.appendChild(assistantDiv);
            messages.scrollTop = messages.scrollHeight;

            let firstChunk = true;
            try {
                const response = await fetch('/api/chat/stream', {
                    method: 'POST',
                    headers: {
                        'Content-Type': 'application/json',
                        'X-Session-Id': getSessionId()
                    },
                    body: JSON.stringify({ message: message, session_id: getSessionId() })
                });

                if (!response.ok) {
                    assistantDiv.innerHTML = 'Error: HTTP ' + response.status;
                    assistantDiv.className = 'chat-message error';
                    return;
                }

                const reader = response.body.getReader();
                const decoder = new TextDecoder();
                let buffer = '';

                while (true) {
                    const { done, value } = await reader.read();
                    if (done) break;
                    buffer += decoder.decode(value, { stream: true });

                    // Parse SSE events (separated by \n\n)
                    const events = buffer.split('\n\n');
                    buffer = events.pop(); // keep incomplete event in buffer

                    for (const event of events) {
                        const dataLine = event.split('\n').find(l => l.startsWith('data: '));
                        if (!dataLine) continue;
                        try {
                            const data = JSON.parse(dataLine.slice(6));
                            if (data.type === 'chunk') {
                                if (firstChunk) {
                                    assistantDiv.textContent = '';
                                    firstChunk = false;
                                }
                                assistantDiv.textContent += data.content;
                                messages.scrollTop = messages.scrollHeight;
                            } else if (data.type === 'error') {
                                assistantDiv.innerHTML = 'Error: ' + escapeHtml(data.error);
                                assistantDiv.className = 'chat-message error';
                            } else if (data.type === 'done') {
                                // If nothing was streamed, use the full response
                                if (firstChunk && data.response) {
                                    assistantDiv.textContent = data.response;
                                    firstChunk = false;
                                }
                            }
                        } catch (e) {
                            console.error('Failed to parse SSE event:', e);
                        }
                    }
                }
            } catch (e) {
                assistantDiv.innerHTML = 'Network error: ' + escapeHtml(e.message);
                assistantDiv.className = 'chat-message error';
            } finally {
                input.disabled = false;
                input.focus();
                messages.scrollTop = messages.scrollHeight;
            }
        }

        async function clearChat() {
            try {
                await fetch('/api/chat', {
                    method: 'POST',
                    headers: {
                        'Content-Type': 'application/json',
                        'X-Session-Id': getSessionId()
                    },
                    body: JSON.stringify({ message: '/clear', reset: true, session_id: getSessionId() })
                });
            } catch (e) {}
            document.getElementById('chatMessages').innerHTML = '<div class="chat-message assistant">Conversation cleared. How can I help you?</div>';
        }

        loadWorkflows();
    </script>
</body>
</html>`
