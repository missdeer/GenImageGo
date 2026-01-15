    // === Custom Prompt Tool ===
    const CustomPromptTool = {
        modal: null, listEl: null, emptyEl: null,
        allPrompts: [], searchTerm: '',
        loading: false,

        init() {
            this.modal = document.getElementById('custom-prompt-modal');
            this.listEl = document.getElementById('custom-prompt-list');
            this.emptyEl = document.getElementById('custom-prompt-empty');
        },

        open() {
            if (!this.modal) this.init();
            closeAllSidebars();
            this.modal.classList.add('active');
            this.loadPrompts();
        },

        close() {
            this.modal.classList.remove('active');
        },

        async loadPrompts() {
            if (this.loading) return;
            this.loading = true;

            try {
                const response = await fetch('/api/user/prompts', {
                    method: 'GET',
                    credentials: 'include'
                });

                if (response.status === 401) {
                    showToast('请先登录', 'warning');
                    this.allPrompts = [];
                    this.render();
                    return;
                }

                if (!response.ok) {
                    throw new Error('加载失败');
                }

                const data = await response.json();
                this.allPrompts = data.prompts || [];
                this.render();
            } catch (e) {
                console.error('Failed to load custom prompts:', e);
                showToast('加载提示词失败', 'error');
                this.allPrompts = [];
                this.render();
            } finally {
                this.loading = false;
            }
        },

        generateUUID() {
            if (typeof crypto !== 'undefined' && crypto.randomUUID) {
                return crypto.randomUUID();
            }
            return 'xxxxxxxx-xxxx-4xxx-yxxx-xxxxxxxxxxxx'.replace(/[xy]/g, function(c) {
                const r = Math.random() * 16 | 0;
                const v = c === 'x' ? r : (r & 0x3 | 0x8);
                return v.toString(16);
            });
        },

        async save() {
            const title = document.getElementById('custom-prompt-title').value.trim();
            const content = document.getElementById('custom-prompt-content').value.trim();
            const editId = document.getElementById('custom-prompt-edit-id').value;

            if (!title) {
                showToast('请输入标题', 'warning');
                return;
            }

            if (title.length > 100) {
                showToast('标题不能超过100个字符', 'warning');
                return;
            }

            if (!content) {
                showToast('请输入提示词内容', 'warning');
                return;
            }

            if (content.length > 10000) {
                showToast('内容不能超过10000个字符', 'warning');
                return;
            }

            try {
                let response;
                if (editId) {
                    response = await fetch('/api/user/prompts/update', {
                        method: 'POST',
                        headers: csrfHeaders({ 'Content-Type': 'application/json' }),
                        credentials: 'include',
                        body: JSON.stringify({ id: parseInt(editId), title, content })
                    });
                } else {
                    const clientId = this.generateUUID();
                    response = await fetch('/api/user/prompts', {
                        method: 'POST',
                        headers: csrfHeaders({ 'Content-Type': 'application/json' }),
                        credentials: 'include',
                        body: JSON.stringify({ client_id: clientId, title, content })
                    });
                }

                if (response.status === 401) {
                    showToast('请先登录', 'warning');
                    return;
                }

                if (!response.ok) {
                    const data = await response.json();
                    throw new Error(data.error || '操作失败');
                }

                showToast(editId ? '更新成功' : '保存成功', 'success');
                this.clearForm();
                await this.loadPrompts();
            } catch (e) {
                console.error('Failed to save prompt:', e);
                showToast(e.message || '保存失败', 'error');
            }
        },

        async delete(id) {
            if (!confirm('确定要删除这条提示词吗？')) return;

            try {
                const response = await fetch('/api/user/prompts/delete', {
                    method: 'POST',
                    headers: csrfHeaders({ 'Content-Type': 'application/json' }),
                    credentials: 'include',
                    body: JSON.stringify({ id })
                });

                if (response.status === 401) {
                    showToast('请先登录', 'warning');
                    return;
                }

                if (!response.ok) {
                    const data = await response.json();
                    throw new Error(data.error || '删除失败');
                }

                showToast('已删除', 'success');
                await this.loadPrompts();
            } catch (e) {
                console.error('Failed to delete prompt:', e);
                showToast(e.message || '删除失败', 'error');
            }
        },

        edit(id) {
            const prompt = this.allPrompts.find(p => p.id === id);
            if (!prompt) return;

            document.getElementById('custom-prompt-title').value = prompt.title;
            document.getElementById('custom-prompt-content').value = prompt.content;
            document.getElementById('custom-prompt-edit-id').value = prompt.id;
            document.getElementById('custom-prompt-form-title').textContent = '编辑提示词';

            const container = this.modal.querySelector('[style*="padding-top: 80px"]');
            if (container) container.scrollTop = 0;
        },

        clearForm() {
            document.getElementById('custom-prompt-title').value = '';
            document.getElementById('custom-prompt-content').value = '';
            document.getElementById('custom-prompt-edit-id').value = '';
            document.getElementById('custom-prompt-form-title').textContent = '新建提示词';
        },

        handleSearch(val) {
            this.searchTerm = val.toLowerCase().trim();
            this.render();
        },

        usePrompt(id, sendDirect = false) {
            const prompt = this.allPrompts.find(p => p.id === id);
            if (!prompt) return;

            this.close();

            const textarea = document.getElementById('user-input');
            if (textarea) {
                textarea.value = prompt.content;
                adjustTextareaHeight();
                checkInput();
                textarea.focus();

                if (sendDirect) {
                    setTimeout(() => {
                        sendMessage();
                    }, 100);
                } else {
                    showToast('提示词已填充', 'success');
                }
            }
        },

        render() {
            this.listEl.innerHTML = '';

            let filtered = this.allPrompts;
            if (this.searchTerm) {
                filtered = this.allPrompts.filter(p => {
                    const title = p.title.toLowerCase();
                    const content = p.content.toLowerCase();
                    return title.includes(this.searchTerm) || content.includes(this.searchTerm);
                });
            }

            if (filtered.length === 0) {
                this.listEl.style.display = 'none';
                this.emptyEl.style.display = 'block';
                return;
            }

            this.listEl.style.display = 'grid';
            this.emptyEl.style.display = 'none';

            filtered.forEach(prompt => {
                const card = document.createElement('div');
                card.className = 'banana-card';

                const preview = prompt.content.substring(0, 100) + (prompt.content.length > 100 ? '...' : '');
                const createdDate = new Date(prompt.created_at).toLocaleDateString('zh-CN');

                card.innerHTML = `
                    <div class="banana-content" style="padding: 16px;">
                        <div class="banana-title" style="margin-bottom: 8px;">${escapeHtml(prompt.title)}</div>
                        <div class="banana-prompt-text" style="font-size: 12px; color: #666; line-height: 1.5; margin-bottom: 12px; display: -webkit-box; -webkit-line-clamp: 3; -webkit-box-orient: vertical; overflow: hidden;">
                            ${escapeHtml(preview)}
                        </div>
                        <div class="banana-footer" style="display: flex; justify-content: space-between; align-items: center; border-top: 1px solid #f0f0f0; padding-top: 12px; margin-top: 12px;">
                            <div class="banana-author" style="font-size: 11px; color: #999;">
                                <svg viewBox="0 0 24 24" width="12" height="12" fill="none" stroke="currentColor" stroke-width="2" style="margin-right: 4px;"><rect x="3" y="4" width="18" height="18" rx="2" ry="2"></rect><line x1="16" y1="2" x2="16" y2="6"></line><line x1="8" y1="2" x2="8" y2="6"></line><line x1="3" y1="10" x2="21" y2="10"></line></svg>
                                ${createdDate}
                            </div>
                            <div class="banana-actions" style="display: flex; gap: 8px;">
                                <div class="banana-icon-btn" title="填充到输入框" onclick="CustomPromptTool.usePrompt(${prompt.id}, false)">
                                    <svg viewBox="0 0 24 24" width="16" height="16" fill="none" stroke="currentColor" stroke-width="2"><path d="M16 4h2a2 2 0 0 1 2 2v14a2 2 0 0 1-2 2H6a2 2 0 0 1-2-2V6a2 2 0 0 1 2-2h2"></path><rect x="8" y="2" width="8" height="4" rx="1" ry="1"></rect></svg>
                                </div>
                                <div class="banana-icon-btn" title="直接发送" onclick="CustomPromptTool.usePrompt(${prompt.id}, true)" style="color: #1a73e8;">
                                    <svg viewBox="0 0 24 24" width="16" height="16" fill="none" stroke="currentColor" stroke-width="2"><line x1="22" y1="2" x2="11" y2="13"></line><polygon points="22 2 15 22 11 13 2 9 22 2"></polygon></svg>
                                </div>
                                <div class="banana-icon-btn" title="编辑" onclick="CustomPromptTool.edit(${prompt.id})">
                                    <svg viewBox="0 0 24 24" width="16" height="16" fill="none" stroke="currentColor" stroke-width="2"><path d="M11 4H4a2 2 0 0 0-2 2v14a2 2 0 0 0 2 2h14a2 2 0 0 0 2-2v-7"></path><path d="M18.5 2.5a2.121 2.121 0 0 1 3 3L12 15l-4 1 1-4 9.5-9.5z"></path></svg>
                                </div>
                                <div class="banana-icon-btn" title="删除" onclick="CustomPromptTool.delete(${prompt.id})" style="color: #d93025;">
                                    <svg viewBox="0 0 24 24" width="16" height="16" fill="none" stroke="currentColor" stroke-width="2"><polyline points="3 6 5 6 21 6"></polyline><path d="M19 6v14a2 2 0 0 1-2 2H7a2 2 0 0 1-2-2V6m3 0V4a2 2 0 0 1 2-2h4a2 2 0 0 1 2 2v2"></path></svg>
                                </div>
                            </div>
                        </div>
                    </div>
                `;

                this.listEl.appendChild(card);
            });
        }
    };
