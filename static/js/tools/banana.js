    // === Banana Prompt Logic ===
       // === Banana Prompt Logic (修复版) ===
    const BananaTool = {
        modal: null, grid: null, loading: null, error: null,
        allData: [], currentFilter: 'all', searchTerm: '',
        init() {
            this.modal = document.getElementById('banana-modal');
            this.grid = document.getElementById('banana-grid');
            this.loading = document.getElementById('banana-loading');
            this.error = document.getElementById('banana-error');
        },
        open() {
            if(!this.modal) this.init();
            closeAllSidebars();
            this.modal.classList.add('active');
            if(this.allData.length === 0) this.fetchData();
        },
        close() { this.modal.classList.remove('active'); },
        async fetchData() {
            this.loading.style.display = 'block';
            this.error.style.display = 'none';
            this.grid.innerHTML = '';
            
            const URLS = [
                'https://raw.githubusercontent.com/glidea/banana-prompt-quicker/refs/heads/main/prompts.json',
                'https://cdn.jsdelivr.net/gh/glidea/banana-prompt-quicker@main/prompts.json',
                'https://fastly.jsdelivr.net/gh/glidea/banana-prompt-quicker@main/prompts.json',
                'http://gh.halonice.com/https://raw.githubusercontent.com/glidea/banana-prompt-quicker/refs/heads/main/prompts.json'
            ];

            let lastError = null;
            for (let i = 0; i < URLS.length; i++) {
                try {
                    const res = await nativeFetch(URLS[i], { timeout: 10000 });

                    if(!res.ok) {
                        throw new Error(`HTTP ${res.status}: 数据加载失败`);
                    }

                    const data = await res.json();

                    if (!Array.isArray(data) || data.length === 0) {
                        throw new Error('数据格式错误或为空');
                    }

                    this.allData = data;
                    this.loading.style.display = 'none';
                    this.render();
                    showToast(`成功加载 ${data.length} 个提示词`, 'success');
                    return;

                } catch(e) {
                    console.warn(`源 ${i + 1} 失败:`, URLS[i], e.message);
                    lastError = e;
                    if (i < URLS.length - 1) continue;
                }
            }

            console.error('所有源均失败:', lastError);
            this.loading.style.display = 'none';
            this.error.style.display = 'block';
            ErrorHandler.show('加载提示词失败', '所有数据源均不可用，请稍后重试');
        },
        filter(type, btnEl) {
            this.currentFilter = type;
            document.querySelectorAll('.banana-tab').forEach(t => t.classList.remove('active'));
            btnEl.classList.add('active');
            this.render();
        },
        handleSearch(val) {
            this.searchTerm = val.toLowerCase().trim();
            this.render();
        },
        render() {
            this.grid.innerHTML = '';
            const filtered = this.allData.filter(item => {
                let tabMatch = true;
                if(this.currentFilter === 'all') {
                    tabMatch = true;
                } else if(this.currentFilter === 'generate') {
                    tabMatch = item.mode === 'generate';
                } else if(this.currentFilter === 'edit') {
                    tabMatch = item.mode === 'edit';
                } else if(this.currentFilter === 'nsfw') {
                    tabMatch = (item.category || '').toLowerCase() === 'nsfw';
                } else if(this.currentFilter === 'study') {
                    tabMatch = (item.category || '').toLowerCase() === '学习';
                } else if(this.currentFilter === 'work') {
                    tabMatch = (item.category || '').toLowerCase() === '工作';
                }
                
                let searchMatch = true;
                if(this.searchTerm) {
                    const s = this.searchTerm;
                    const title = (item.title || '').toLowerCase();
                    const prompt = (item.prompt || '').toLowerCase();
                    const category = (item.category || '').toLowerCase();
                    if(!title.includes(s) && !prompt.includes(s) && !category.includes(s)) searchMatch = false;
                }
                return tabMatch && searchMatch;
            });
            if(filtered.length === 0) {
                this.grid.innerHTML = `<div style="text-align:center; color:#999; grid-column:1/-1; padding:40px;"><svg width="48" height="48" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5" style="margin:0 auto 12px; display:block; opacity:0.5;"><circle cx="11" cy="11" r="8"/><line x1="21" y1="21" x2="16.65" y2="16.65"/></svg>未找到相关提示词</div>`;
                return;
            }
            filtered.forEach(item => {
                const card = document.createElement('div');
                card.className = 'banana-card';
                const modeTagClass = item.mode === 'generate' ? 'mode-generate' : 'mode-edit';
                const safePrompt = encodeURIComponent(item.prompt);
                const safeTitle = encodeURIComponent(item.title);
                card.innerHTML = `<div class="banana-preview-box"><img src="${item.preview}" class="banana-img" loading="lazy" onerror="this.src='https://placehold.co/600x400/e2e8f0/94a3b8?text=No+Preview'"><div class="banana-tags"><span class="banana-tag">${item.category}</span><span class="banana-tag ${modeTagClass}">${item.mode}</span></div></div><div class="banana-content"><div class="banana-title">${item.title}</div><div class="banana-prompt-box" onclick="BananaTool.copy('${safePrompt}')"><div class="banana-prompt-text">${escapeHtml(item.prompt)}</div><div class="banana-prompt-tip"><span>点击复制</span></div></div><div class="banana-footer"><div class="banana-author"><svg viewBox="0 0 24 24" width="14" height="14" fill="none" stroke="currentColor" stroke-width="2"><path d="M20 21v-2a4 4 0 0 0-4-4H8a4 4 0 0 0-4 4v2"></path><circle cx="12" cy="7" r="4"></circle></svg> ${item.author ? item.author.split('@')[0] : 'Unknown'}</div><div class="banana-actions">${item.link ? `<a href="${item.link}" target="_blank" class="banana-icon-btn" title="查看原链接"><svg viewBox="0 0 24 24" width="16" height="16" fill="none" stroke="currentColor" stroke-width="2"><path d="M18 13v6a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2V8a2 2 0 0 1 2-2h6"></path><polyline points="15 3 21 3 21 9"></polyline><line x1="10" y1="14" x2="21" y2="3"></line></svg></a>` : ''}<div class="banana-icon-btn" title="填充到输入框" onclick="BananaTool.sendToInput('${safePrompt}')" style="color:#1a73e8;"><svg viewBox="0 0 24 24" width="16" height="16" fill="none" stroke="currentColor" stroke-width="2"><path d="M11 4H4a2 2 0 0 0-2 2v14a2 2 0 0 0 2 2h14a2 2 0 0 0 2-2v-7"></path><path d="M18.5 2.5a2.121 2.121 0 0 1 3 3L12 15l-4 1 1-4 9.5-9.5z"></path></svg></div><div class="banana-icon-btn" title="保存到我的提示词" onclick="BananaTool.saveToCustom('${safeTitle}','${safePrompt}')" style="color:#ea4335;"><svg viewBox="0 0 24 24" width="16" height="16" fill="none" stroke="currentColor" stroke-width="2"><path d="M19 21H5a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h11l5 5v11a2 2 0 0 1-2 2z" />
                    <polyline points="17 21 17 13 7 13 7 21" />
                    <polyline points="7 3 7 8 15 8" /></svg></div></div></div></div>`;
                this.grid.appendChild(card);
            });
        },
        copy(encodedText) {
            const text = decodeURIComponent(encodedText);
            if (navigator.clipboard && navigator.clipboard.writeText) {
                navigator.clipboard.writeText(text).then(() => {
                    showToast('提示词已复制！');
                }).catch(() => {
                    this.fallbackCopy(text);
                });
            } else {
                this.fallbackCopy(text);
            }
        },
        fallbackCopy(text) {
            const textArea = document.createElement("textarea");
            textArea.value = text;
            textArea.style.position = "fixed";
            textArea.style.left = "-9999px";
            document.body.appendChild(textArea);
            textArea.focus();
            textArea.select();
            try {
                const successful = document.execCommand('copy');
                if(successful) showToast('提示词已复制！');
                else alert('复制失败');
            } catch (err) {
                alert('浏览器不支持自动复制');
            }
            document.body.removeChild(textArea);
        },
        // 发送到对话框
        sendToInput(encodedText, shouldSend = false) {
            const text = decodeURIComponent(encodedText);
            const textarea = document.getElementById('user-input');
            if (textarea) {
                textarea.value = text;
                adjustTextareaHeight();
                checkInput();
                textarea.focus();

                // 关闭模态框
                this.close();

                // 如果需要直接发送
                if (shouldSend) {
                    setTimeout(() => {
                        sendMessage();
                    }, 100);
                } else {
                    showToast('提示词已填充到输入框', 'success');
                }
            }
        },
        // 保存到我的提示词
        saveToCustom(encodedTitle, encodedPrompt) {
            const title = decodeURIComponent(encodedTitle);
            const prompt = decodeURIComponent(encodedPrompt);

            // 检查是否已存在
            if (!CustomPromptTool.allPrompts) {
                CustomPromptTool.init();
            }

            const exists = CustomPromptTool.allPrompts.some(p =>
                p.title === title || p.content === prompt
            );

            if (exists) {
                showToast('该提示词已存在于我的提示词中', 'warning', 2000);
                return;
            }

            // 添加到我的提示词
            CustomPromptTool.allPrompts.unshift({
                id: 'prompt_' + Date.now(),
                title: title,
                content: prompt,
                createdAt: Date.now(),
                updatedAt: Date.now()
            });

            CustomPromptTool.savePrompts();
            showToast('已保存到我的提示词 ✓', 'success', 2000);
        }
    };
