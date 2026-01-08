let UI = {};
    const state={images:[],resolution:'4K',aspectRatio:'auto',useStreaming:false,useContext:false,contextCount:5};
    
    window.initializeApp = async () => {
        // Initialize UI elements after they are loaded into the DOM
        UI = {
            chatHistory: document.getElementById('chat-history'),
            emptyState: document.getElementById('empty-state'),
            sessionList: document.getElementById('session-list'),
            textarea: document.getElementById('user-input'),
            fileInput: document.getElementById('file-input'),
            previewArea: document.getElementById('preview-area'),
            sendBtn: document.getElementById('send-btn'),
            enhanceBtn: document.getElementById('enhance-btn')
        };

        // Initialize Tools
        SlicerTool.init();
        BananaTool.init();
        CustomPromptTool.init();
        FileSystemManager.init();
        
        // Initialize DB and Sessions
        await initDB();
        await renderSessionList();
        const sessions = await getAllSessions();
        if (sessions.length > 0) await loadSession(sessions[0].id);
        else await createNewSession();

        // UI Event Listeners
        const streamToggle = document.getElementById('stream-toggle');
        if (streamToggle) {
            streamToggle.checked = localStorage.getItem('use_streaming') === 'true';
            state.useStreaming = streamToggle.checked;
            streamToggle.addEventListener('change', () => {
                state.useStreaming = streamToggle.checked;
                localStorage.setItem('use_streaming', streamToggle.checked);
            });
        }

        const contextToggle = document.getElementById('context-toggle');
        const contextCount = document.getElementById('context-count');
        if (contextToggle && contextCount) {
            contextToggle.checked = localStorage.getItem('use_context') === 'true';
            state.useContext = contextToggle.checked;
            state.contextCount = parseInt(localStorage.getItem('context_count') || '5');
            contextCount.value = state.contextCount;
            contextToggle.addEventListener('change', () => {
                state.useContext = contextToggle.checked;
                localStorage.setItem('use_context', contextToggle.checked);
            });
            contextCount.addEventListener('change', () => {
                state.contextCount = parseInt(contextCount.value);
                localStorage.setItem('context_count', contextCount.value);
            });
        }

        // Textarea Events
        if (UI.textarea) {
            UI.textarea.addEventListener('input', function() { checkInput(); adjustTextareaHeight(); });
            UI.textarea.addEventListener('keydown', (e) => { if (e.key === 'Enter' && !e.shiftKey) { e.preventDefault(); sendMessage(); } });
            
            // Paste Support
            UI.textarea.addEventListener('paste', async (e) => {
                const items = e.clipboardData?.items;
                if (!items) return;
                for (let item of items) {
                    if (item.type.startsWith('image/')) {
                        e.preventDefault();
                        const file = item.getAsFile();
                        if (file) {
                            await handleFiles([file]);
                            showToast('已粘贴图片', 'success');
                        }
                    }
                }
            });
        }

        // Drag and Drop Support
        const inputWrapper = document.querySelector('.input-wrapper');
        const inputContainerOuter = document.querySelector('.input-container-outer');
        [inputWrapper, inputContainerOuter].forEach(element => {
            if (!element) return;
            element.addEventListener('dragover', (e) => { e.preventDefault(); e.stopPropagation(); inputWrapper.classList.add('drag-over'); });
            element.addEventListener('dragleave', (e) => { e.preventDefault(); e.stopPropagation(); if (!inputWrapper.contains(e.relatedTarget)) inputWrapper.classList.remove('drag-over'); });
            element.addEventListener('drop', async (e) => {
                e.preventDefault(); e.stopPropagation(); inputWrapper.classList.remove('drag-over');
                const files = Array.from(e.dataTransfer.files).filter(f => f.type.startsWith('image/'));
                if (files.length > 0) { await handleFiles(files); showToast(`已添加 ${files.length} 张图片`, 'success'); } 
                else if (e.dataTransfer.files.length > 0) showToast('请拖拽图片文件', 'warning');
            });
        });

        // Chat Container Drag Support
        const chatContainer = document.querySelector('.chat-container');
        if (chatContainer) {
            chatContainer.addEventListener('dragover', (e) => {
                const files = Array.from(e.dataTransfer.items).filter(item => item.kind === 'file' && item.type.startsWith('image/'));
                if (files.length > 0) { e.preventDefault(); e.stopPropagation(); inputWrapper.classList.add('drag-over'); }
            });
            chatContainer.addEventListener('drop', async (e) => {
                const files = Array.from(e.dataTransfer.files).filter(f => f.type.startsWith('image/'));
                if (files.length > 0) {
                    e.preventDefault(); e.stopPropagation(); inputWrapper.classList.remove('drag-over');
                    await handleFiles(files); showToast(`已添加 ${files.length} 张图片`, 'success');
                }
            });
        }

        // Settings Buttons
        document.querySelectorAll('.res-btn').forEach(btn => btn.addEventListener('click', () => {
            document.querySelectorAll('.res-btn').forEach(b => b.classList.remove('active'));
            btn.classList.add('active'); state.resolution = btn.dataset.val;
        }));
        document.querySelectorAll('.ratio-card').forEach(card => card.addEventListener('click', () => {
            document.querySelectorAll('.ratio-card').forEach(c => c.classList.remove('active'));
            card.classList.add('active'); state.aspectRatio = card.dataset.val;
        }));

        adjustTextareaHeight();
        checkInput();
    };

    function activateStickerMode(){createNewSession("表情包制作").then(()=>{
        const stickerPrompt="为我生成图中角色的绘制 Q 版的，LINE 风格的半身像表情包，注意头饰要正确\n彩色手绘风格，使用 4x6 布局，涵盖各种各样的常用聊天语句，或是一些有关的娱乐 meme\n其他需求：不要原图复制。所有标注为手写简体中文。    ";
        UI.textarea.value=stickerPrompt;
        state.resolution='4K';
        document.querySelectorAll('.res-btn').forEach(b=>b.classList.remove('active'));
        document.querySelector('.res-btn[data-val="4K"]').classList.add('active');
        state.aspectRatio='16:9';
        document.querySelectorAll('.ratio-card').forEach(c=>c.classList.remove('active'));
        document.querySelector('.ratio-card[data-val="16:9"]').classList.add('active');
        alert("已进入表情包模式！\n请点击输入框左侧图标上传一张角色参考图，然后点击发送。");
        adjustTextareaHeight();
        checkInput();
        if(window.innerWidth<=768)closeAllSidebars()
    })}
    
    async function renderSessionList(){const sessions=await getAllSessions();if(UI.sessionList){UI.sessionList.innerHTML='';sessions.forEach(s=>{const div=document.createElement('div');div.className=`session-item ${s.id===currentSessionId?'active':''}`;if(activeGenerations.has(s.id))div.classList.add('generating');div.innerHTML=`<div style="display:flex; align-items:center; overflow:hidden; width:100%;"><span class="session-loading">⏳</span><span style="overflow:hidden; text-overflow:ellipsis;">${escapeHtml(s.title)}</span></div><div class="session-delete" onclick="event.stopPropagation(); removeSession(${s.id})">×</div>`;div.onclick=()=>loadSession(s.id);UI.sessionList.appendChild(div)})}}
    
    async function loadSession(sessionId){currentSessionId=sessionId;if(UI.chatHistory){UI.chatHistory.innerHTML='';UI.emptyState.style.display='none';}BlobManager.cleanup();renderSessionList();const messages=await getSessionMessages(sessionId);if(messages.length===0){if(UI.chatHistory)UI.chatHistory.appendChild(UI.emptyState);if(UI.emptyState)UI.emptyState.style.display='flex'}else{messages.forEach(msg=>appendMessageToUI(msg.role,msg.rawHtml,msg.content,msg.images,msg.id));if(UI.chatHistory)UI.chatHistory.scrollTop=UI.chatHistory.scrollHeight}if(activeGenerations.has(sessionId))appendMessageToUI('bot','<div class="loading-spinner" id="temp-loading" style="margin-left:20px;"></div>');if(window.innerWidth<=768)closeAllSidebars()}async function createNewSession(title="新对话"){const id=await createSession(title);await loadSession(id)}async function removeSession(id){if(!confirm('确定删除此对话？'))return;await deleteSession(id);if(id===currentSessionId){const sessions=await getAllSessions();if(sessions.length>0)await loadSession(sessions[0].id);else await createNewSession()}else{renderSessionList()}}

    // 清除全部对话
    async function clearAllSessions() {
        if (!confirm('确定要清除全部对话记录吗？此操作不可恢复！')) return;

        try {
            const sessions = await getAllSessions();

            // 删除所有对话
            for (const session of sessions) {
                await deleteSession(session.id);
            }

            // 创建新对话
            await createNewSession();

            showToast('✅ 已清除全部对话记录', 'success', 2000);
        } catch (error) {
            console.error('清除对话失败:', error);
            showToast('清除失败: ' + error.message, 'error');
        }
    }

    function useAsReference(base64){const mime="image/jpeg";const fullB64=base64.startsWith('data:')?base64:`data:${mime};base64,${base64}`;const rawBase64=fullB64.split(',')[1];state.images.push({base64:rawBase64,mimeType:mime,preview:base64ToBlobUrl(fullB64)});renderPreviews();checkInput();UI.textarea.focus();window.scrollTo(0,document.body.scrollHeight)}

    async function enhancePrompt() {
        if (!UI.textarea) return;
        const text = UI.textarea.value.trim();
        if (!text) {
            showToast('请先输入提示词', 'error');
            return;
        }
        if (UI.enhanceBtn) UI.enhanceBtn.classList.add('loading');
        UI.textarea.disabled = true;
        try {
            console.log('enhancePrompt: sending request with text:', text);
            const resp = await fetch('/enhance-prompt', {
                method: 'POST',
                headers: { 'Content-Type': 'text/plain' },
                body: text
            });
            console.log('enhancePrompt: response status:', resp.status, 'content-type:', resp.headers.get('content-type'));
            if (!resp.ok) {
                const errText = await resp.text();
                console.log('enhancePrompt: error response:', errText);
                throw new Error(errText || resp.statusText);
            }
            const result = await resp.text();
            console.log('enhancePrompt: result text:', result);
            UI.textarea.value = result;
            adjustTextareaHeight();
            checkInput();
            showToast('提示词已优化', 'success');
        } catch (e) {
            console.error('enhancePrompt error:', e);
            showToast('优化失败: ' + e.message, 'error');
        } finally {
            if (UI.enhanceBtn) UI.enhanceBtn.classList.remove('loading');
            UI.textarea.disabled = false;
            UI.textarea.focus();
        }
    }

    async function sendMessage(){
        if(!UI.textarea) return;
        const rawText=UI.textarea.value||'';
        const text=rawText.trim();
        const rawImages=Array.isArray(state.images)?state.images.slice():[];
        const imagesBase64=rawImages.map(img=>{
            if(typeof img==='string') return img.startsWith('data:')?img.split(',')[1]:img;
            if(!img||typeof img!=='object') return null;
            if(typeof img.base64==='string') return img.base64.startsWith('data:')?img.base64.split(',')[1]:img.base64;
            return null;
        }).filter(Boolean);

        if(!text&&imagesBase64.length===0) return;

        if(!currentSessionId) await createNewSession();
        const sessionId=currentSessionId;

        if(UI.emptyState&&UI.emptyState.parentElement===UI.chatHistory){
            UI.emptyState.style.display='none';
            UI.chatHistory.removeChild(UI.emptyState);
        }

        const displayText=text|| (imagesBase64.length>0?'图片':'');
        const escapedText=escapeHtml(displayText).replace(/\n/g,'<br>');
        const userHtml=`<div class="msg-content">${escapedText}</div>`;

        let userMsgId=null;
        try{
            userMsgId=await saveMessage(sessionId,'user',text,imagesBase64,userHtml);
        }catch(e){
            console.error('Failed to save message:',e);
        }

        appendMessageToUI('user',userHtml,text,rawImages,userMsgId);

        UI.textarea.value='';
        state.images=[];
        renderPreviews();
        checkInput();
        adjustTextareaHeight();

        if(text){
            try{
                const sessions=await getAllSessions();
                const currentSession=sessions.find(s=>s.id===sessionId);
                if(currentSession&&currentSession.title==='新对话'){
                    const title=text.length>20?text.slice(0,20)+'...':text;
                    updateSessionTitle(sessionId,title);
                    renderSessionList();
                }
            }catch(e){
                console.error('Failed to update session title:',e);
            }
        }

        activeGenerations.add(sessionId);
        renderSessionList();

        const progressId=`gen-progress-${sessionId}-${Date.now()}`;
        const loadingHtml=SmartProgressBar.createHTML(progressId);
        const loadingDiv=appendMessageToUI('bot',loadingHtml,null,[],null);
        SmartProgressBar.start(progressId,state.resolution,imagesBase64.length>0);

        processGeneration(text,imagesBase64,loadingDiv,sessionId,progressId);
    }

    function handleEdit(rawText, rawImages){
        if(!UI.textarea) return;
        UI.textarea.value=rawText||'';
        state.images=[];
        if(Array.isArray(rawImages)){
            rawImages.forEach(img=>{
                if(typeof img==='string'){
                    const dataUrl=img.startsWith('data:')?img:`data:image/jpeg;base64,${img}`;
                    const base64=dataUrl.split(',')[1]||'';
                    if(base64) state.images.push({base64:base64,mimeType:'image/jpeg',preview:base64ToBlobUrl(dataUrl)});
                    return;
                }
                if(img&&typeof img==='object'){
                    const mimeType=img.mimeType||'image/jpeg';
                    const base64=typeof img.base64==='string'?(img.base64.startsWith('data:')?img.base64.split(',')[1]:img.base64):'';
                    const dataUrl=base64?`data:${mimeType};base64,${base64}`:'';
                    const preview=img.preview|| (dataUrl?base64ToBlobUrl(dataUrl):'');
                    if(base64&&preview) state.images.push({base64:base64,mimeType:mimeType,preview:preview});
                }
            });
        }
        renderPreviews();
        checkInput();
        adjustTextareaHeight();
        UI.textarea.focus();
        window.scrollTo(0,document.body.scrollHeight);
    }

    function handleRegenerate(rawText, rawImages){
        handleEdit(rawText,rawImages);
        setTimeout(()=>sendMessage(),0);
    }

    async function handleDeleteMessage(messageId){
        if(!messageId) return;
        if(!confirm('确定删除此消息？')) return;
        try{
            await deleteMessage(messageId);
            const row=document.querySelector(`[data-message-id="${messageId}"]`);
            if(row&&row.parentElement) row.parentElement.removeChild(row);
            if(UI.chatHistory&&UI.chatHistory.querySelectorAll('.message-row').length===0){
                if(UI.emptyState){
                    UI.emptyState.style.display='flex';
                    UI.chatHistory.appendChild(UI.emptyState);
                }
            }
        }catch(e){
            console.error('Failed to delete message:',e);
            showToast('删除失败: '+e.message,'error');
        }
    }

    async function urlToRef(url){
        if(!url) return;
        try{
            const res=await nativeFetch(url);
            if(!res.ok) throw new Error(`HTTP ${res.status}`);
            const blob=await res.blob();
            const dataUrl=await new Promise((resolve,reject)=>{
                const reader=new FileReader();
                reader.onload=()=>resolve(reader.result);
                reader.onerror=()=>reject(new Error('Failed to read image'));
                reader.readAsDataURL(blob);
            });
            useAsReference(String(dataUrl));
        }catch(e){
            console.error('Failed to load image URL:',e);
            showToast('无法读取图片链接: '+e.message,'error');
        }
    }
    
    function appendMessageToUI(role, html, rawText = null, rawImages = [], messageId = null) {
        if(!UI.chatHistory) return null;
        const div = document.createElement('div');
        div.className = `message-row ${role}`;
        if (messageId) div.setAttribute('data-message-id', messageId);
        let finalContentHtml = html;
        if (role === 'bot') {
            finalContentHtml = `<div style="display:flex; flex-direction:column; width:100%; align-items:flex-start;">${html}</div>`;
            // 为Bot消息添加操作按钮（只保留删除按钮）
            const botActionsHtml = `<div class="msg-actions" style="justify-content:flex-start;">${messageId ? `<div class="action-btn" onclick="handleDeleteMessage(${messageId})" style="color:#d93025"><svg viewBox="0 0 24 24" width="12" height="12" fill="none" stroke="currentColor" stroke-width="2"><polyline points="3 6 5 6 21 6"></polyline><path d="M19 6v14a2 2 0 0 1-2 2H7a2 2 0 0 1-2-2V6m3 0V4a2 2 0 0 1 2-2h4a2 2 0 0 1 2 2v2"></path></svg> 删除</div>` : ''}</div>`;
            finalContentHtml += botActionsHtml;
        }
        let finalHtml = `<div class="message-bubble-container">${finalContentHtml}</div>`;
        if (role === 'user') {
            if (rawImages && rawImages.length > 0) {
                let imgGridHtml = `<div style="display:flex; gap:5px; flex-wrap:wrap; justify-content:flex-end; margin-bottom:5px; width:100%">`;
                rawImages.forEach(imgData => {
                    let src = ''; if (typeof imgData === 'object' && imgData.preview) { src = imgData.preview; } else if (typeof imgData === 'string') { if (imgData.startsWith('data:')) src = imgData; else src = `data:image/jpeg;base64,${imgData}`; } 
                    if(src) imgGridHtml += `<img src="${src}" class="generated-image" style="width:60px; height:60px; object-fit:cover; border-radius:8px;">`;
                });
                imgGridHtml += `</div>`; finalHtml = `<div style="display:flex; flex-direction:column; align-items:flex-end; width:100%">${imgGridHtml}${finalHtml}</div>`;
            }
            const escapedRawText = rawText ? escapeHtml(rawText) : '';
            let actionsHtml = `<div class="msg-actions"><div class="action-btn" onclick="copyText(this)" data-text="${escapedRawText}"><svg viewBox="0 0 24 24" width="12" height="12" fill="none" stroke="currentColor" stroke-width="2"><rect x="9" y="9" width="13" height="13" rx="2" ry="2"></rect><path d="M5 15H4a2 2 0 0 1-2-2V4a2 2 0 0 1 2-2h9a2 2 0 0 1 2 2v1"></path></svg> 复制</div><div class="action-btn" onclick='handleEdit(${JSON.stringify(rawText||"")}, ${JSON.stringify(rawImages||[])})'><svg viewBox="0 0 24 24" width="12" height="12" fill="none" stroke="currentColor" stroke-width="2"><path d="M11 4H4a2 2 0 0 0-2 2v14a2 2 0 0 0 2 2h14a2 2 0 0 0 2-2v-7"></path><path d="M18.5 2.5a2.121 2.121 0 0 1 3 3L12 15l-4 1 1-4 9.5-9.5z"></path></svg> 编辑</div><div class="action-btn" onclick='handleRegenerate(${JSON.stringify(rawText||"")}, ${JSON.stringify(rawImages||[])})'><svg viewBox="0 0 24 24" width="12" height="12" fill="none" stroke="currentColor" stroke-width="2"><polyline points="23 4 23 10 17 10"></polyline><path d="M20.49 15a9 9 0 1 1-2.12-9.36L23 10"></path></svg> 重新生成</div>${messageId ? `<div class="action-btn" onclick="handleDeleteMessage(${messageId})" style="color:#d93025"><svg viewBox="0 0 24 24" width="12" height="12" fill="none" stroke="currentColor" stroke-width="2"><polyline points="3 6 5 6 21 6"></polyline><path d="M19 6v14a2 2 0 0 1-2 2H7a2 2 0 0 1-2-2V6m3 0V4a2 2 0 0 1 2-2h4a2 2 0 0 1 2 2v2"></path></svg> 删除</div>` : ''}</div>`;
            finalHtml += actionsHtml;
        }
        div.innerHTML = finalHtml; div.querySelectorAll('img.generated-image').forEach(img => img.onclick = () => openLightbox(img.src)); UI.chatHistory.appendChild(div); UI.chatHistory.scrollTop = UI.chatHistory.scrollHeight; return div;
    }

    async function processGeneration(text,imagesBase64,loadingDiv,sessionId,progressId){
        try{
            let data;
            // Build contents array for Gemini API
            let contents = [];
            let contextImages = []; // 收集上下文中的图片

            // If context enabled, get history messages
            if (state.useContext && state.contextCount > 0) {
                const historyMessages = await getSessionMessages(sessionId);
                const recentMessages = historyMessages.slice(-state.contextCount * 2 - 1, -1);
                console.log('📖 读取历史消息，总数:', historyMessages.length, '使用:', recentMessages.length);

                recentMessages.forEach(msg => {
                    const parts = [];
                    if (msg.content) {
                        parts.push({ text: msg.content });
                    }
                    // 收集历史图片，但不放在历史消息中
                    if (msg.images && msg.images.length > 0) {
                        console.log('📸 消息包含图片，数量:', msg.images.length, 'role:', msg.role);
                        msg.images.forEach(b64 => {
                            contextImages.push(b64); // 收集到contextImages数组
                        });
                    }
                    if (parts.length > 0) {
                        contents.push({
                            role: msg.role === 'user' ? 'user' : 'model',
                            parts: parts
                        });
                    }
                });
            }

            // Add current message with all images (context + current)
            const currentParts = text ? [{ text }] : [{ text: "Generate image" }];
            // 先添加上下文中的历史图片
            contextImages.forEach(b64 => {
                currentParts.push({ inline_data: { mime_type: 'image/jpeg', data: b64 } });
            });
            // 再添加当前上传的图片
            imagesBase64.forEach(b64 => {
                currentParts.push({ inline_data: { mime_type: 'image/jpeg', data: b64 } });
            });
            contents.push({ role: "user", parts: currentParts });

            const generationConfig = { responseModalities: ["TEXT", "IMAGE"], imageConfig: { imageSize: state.resolution } };
            if (state.aspectRatio && state.aspectRatio !== 'auto') generationConfig.imageConfig.aspectRatio = state.aspectRatio;
            const payload = { contents: contents, generationConfig: generationConfig };

            // 构建请求 URL
            const baseHost = window.location.origin;
            const requestUrl = `${baseHost.replace(new RegExp('/$'),'')}/generate-image`;

            // 构建请求 headers
            const requestHeaders = {
                'Content-Type': 'application/json'
            };

            const res = await nativeFetch(requestUrl, {
                method: 'POST',
                headers: requestHeaders,
                body: JSON.stringify(payload)
            });
            data = await res.json();
            activeGenerations.delete(sessionId);
            renderSessionList();
            if (!res.ok) throw new Error(JSON.stringify(data));

            let botInnerHtml='';
            let generatedImages = []; // 收集生成的图片base64数据
            if(data.candidates?.[0]?.content?.parts){
                data.candidates[0].content.parts.forEach(part=>{
                    if(part.inlineData&&part.inlineData.mimeType.startsWith('image/')){
                        const fullBase64=`data:${part.inlineData.mimeType};base64,${part.inlineData.data}`;
                        generatedImages.push(part.inlineData.data); // 保存纯base64数据
                        console.log('🎨 收集到生成的图片，base64长度:', part.inlineData.data.length);
                        const now=new Date();
                        const filename=`gemini_${now.getTime()}.png`;

                        // 自动保存到本地目录
                        if (FileSystemManager.isEnabled && FileSystemManager.directoryHandle) {
                            console.log('🎨 图片生成完成，开始自动保存...');
                            FileSystemManager.saveImageToDirectory(fullBase64, filename).then(success => {
                                if (success) {
                                    console.log('✅ 图片已自动保存到本地目录');
                                    showToast(`✅ 图片已保存: ${filename}`, 'success', 2000);
                                }
                            });
                        }

                        botInnerHtml+=`<div class="msg-content" style="padding:0"><div class="img-result-group"><img class="generated-image" src="${fullBase64}" data-filename="${filename}"><div class="btn-group"><div class="tool-btn download" onclick='downloadImage("${fullBase64}", "${filename}")'><svg viewBox="0 0 24 24" width="16" height="16" fill="none" stroke="currentColor" stroke-width="2"><path d="M21 15v4a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2v-4"/><polyline points="7 10 12 15 17 10"/><line x1="12" y1="15" x2="12" y2="3"/></svg> 下载原图</div><div class="tool-btn" onclick='useAsReference("${fullBase64}")'><svg viewBox="0 0 24 24" width="16" height="16" fill="none" stroke="currentColor" stroke-width="2"><path d="M21 15v4a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2v-4"/><polyline points="17 8 12 3 7 8"/><line x1="12" y1="3" x2="12" y2="15"/></svg> 设为参考图</div><div class="tool-btn slice-btn" onclick='SlicerTool.open("${fullBase64}")'><svg viewBox="0 0 24 24" width="16" height="16" fill="none" stroke="currentColor" stroke-width="2"><path d="M6 3L6 21"/><path d="M18 3L18 21"/><path d="M2 12L22 12"/></svg> 切割/表情包</div></div></div></div>`;
                    } else if(part.text){
                        let textContent = part.text;
                        let imagesHtml = '';
                        const imgRegex = /!\[([^\]]*)\]\(((?:https?:|data:image\/)[^)]+)\)/g;
                        let match;

                        while ((match = imgRegex.exec(textContent)) !== null) {
                            const url = match[2];
                            const filename = `image_${Date.now()}_${Math.floor(Math.random()*1000)}.png`;
                            const safeUrl = url;
                            const isBase64 = safeUrl.startsWith('data:');

                            // 提取base64数据并保存
                            if (isBase64) {
                                const base64Data = safeUrl.split(',')[1]; // 提取纯base64部分
                                if (base64Data) {
                                    generatedImages.push(base64Data);
                                    console.log('🎨 收集到Markdown图片，base64长度:', base64Data.length);
                                }
                            }

                            // 自动保存到本地目录（仅 base64 图片）
                            if (isBase64 && FileSystemManager.isEnabled && FileSystemManager.directoryHandle) {
                                console.log('🎨 Markdown 图片生成完成，开始自动保存...');
                                FileSystemManager.saveImageToDirectory(safeUrl, filename).then(success => {
                                    if (success) {
                                        console.log('✅ Markdown 图片已自动保存到本地目录');
                                        showToast(`✅ 图片已保存: ${filename}`, 'success', 2000);
                                    }
                                });
                            }

                            const refAction = isBase64 ? `useAsReference("${safeUrl}")` : `urlToRef("${safeUrl}")`;
                            imagesHtml += `<div class="msg-content" style="padding:0"><div class="img-result-group"><img class="generated-image" src="${safeUrl}" crossorigin="anonymous" onerror="this.onerror=null;this.src='${safeUrl}';"><div class="btn-group"><a class="tool-btn download" href="${safeUrl}" target="_blank" download="${filename}"><svg viewBox="0 0 24 24" width="16" height="16" fill="none" stroke="currentColor" stroke-width="2"><path d="M21 15v4a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2v-4"/><polyline points="7 10 12 15 17 10"/><line x1="12" y1="15" x2="12" y2="3"/></svg> 打开/下载</a><div class="tool-btn" onclick='${refAction}'><svg viewBox="0 0 24 24" width="16" height="16" fill="none" stroke="currentColor" stroke-width="2"><path d="M21 15v4a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2v-4"/><polyline points="17 8 12 3 7 8"/><line x1="12" y1="3" x2="12" y2="15"/></svg> 设为参考图</div><div class="tool-btn slice-btn" onclick='SlicerTool.open("${safeUrl}")'><svg viewBox="0 0 24 24" width="16" height="16" fill="none" stroke="currentColor" stroke-width="2"><path d="M6 3L6 21"/><path d="M18 3L18 21"/><path d="M2 12L22 12"/></svg> 切割/表情包</div></div></div></div>`;
                        }

                        textContent = textContent.replace(imgRegex, '');
                        if (textContent.trim()) {
                            botInnerHtml += `<div class="msg-content" style="padding:0; width:100%"><details class="thought-box"><summary>Thinking / Output</summary><div class="thought-content">${escapeHtml(textContent)}</div></details></div>`;
                        }
                        botInnerHtml += imagesHtml;
                    }
                })
            }

            if(botInnerHtml){
                console.log('💾 保存bot消息（仅图片），图片数量:', generatedImages.length);
                const botMsgId = await saveMessage(sessionId,'bot','Image Generated',generatedImages,botInnerHtml);
                if(sessionId===currentSessionId){
                    // 完成进度条
                    if(progressId) {
                        SmartProgressBar.complete(progressId, () => {
                            if(loadingDiv) loadingDiv.remove();
                        });
                    } else {
                        if(loadingDiv) loadingDiv.remove();
                    }
                    const tempLoading=document.getElementById('temp-loading');
                    if(tempLoading)tempLoading.parentElement.remove();
                    const loadingSpinners = document.querySelectorAll('.loading-spinner');
                    loadingSpinners.forEach(spinner => {
                        if(spinner.parentElement && spinner.parentElement.textContent.includes('正在加载图片')) {
                            spinner.parentElement.remove();
                        }
                    });

                    appendMessageToUI('bot',botInnerHtml,'Image Generated',[],botMsgId);
                }
            }

        }catch(e){
            // 停止进度条
            if(progressId) SmartProgressBar.stop(progressId);

            activeGenerations.delete(sessionId); renderSessionList(); let msg=e.message; try{const jsonErr=JSON.parse(e.message);if(jsonErr.error&&jsonErr.error.message)msg=jsonErr.error.message}catch(_){} const errorHtml=`<div class="msg-content" style="color:#d93025">❌ Error: ${escapeHtml(msg)}</div>`; const errorMsgId = await saveMessage(sessionId,'bot','Error',[],errorHtml); if(sessionId===currentSessionId){ if(loadingDiv)loadingDiv.remove(); appendMessageToUI('bot',errorHtml,'Error',[],errorMsgId) }
        }
    }

    function adjustTextareaHeight(){
        if(!UI.textarea) return;
        UI.textarea.style.height='auto';
        const maxHeight=150;
        UI.textarea.style.height=Math.min(UI.textarea.scrollHeight,maxHeight)+'px';
    }

    async function compressImage(file){
        const dataUrl=await new Promise((resolve,reject)=>{
            const reader=new FileReader();
            reader.onload=()=>resolve(reader.result);
            reader.onerror=()=>reject(new Error('Failed to read image'));
            reader.readAsDataURL(file);
        });
        return new Promise(resolve=>{
            const img=new Image();
            img.onload=()=>{
                const maxDim=2048;
                let targetW=img.width;
                let targetH=img.height;
                const maxSide=Math.max(targetW,targetH);
                if(maxSide>maxDim){
                    const scale=maxDim/maxSide;
                    targetW=Math.round(targetW*scale);
                    targetH=Math.round(targetH*scale);
                }
                const canvas=document.createElement('canvas');
                canvas.width=targetW;
                canvas.height=targetH;
                const ctx=canvas.getContext('2d');
                if(!ctx){
                    const base64=dataUrl.split(',')[1]||'';
                    resolve({base64:base64,mimeType:file.type||'image/jpeg',preview:base64ToBlobUrl(dataUrl)});
                    return;
                }
                ctx.drawImage(img,0,0,targetW,targetH);
                let outUrl='';
                try{
                    outUrl=canvas.toDataURL('image/jpeg',0.85);
                }catch(_){
                    outUrl=dataUrl;
                }
                const base64=outUrl.split(',')[1]||'';
                resolve({base64:base64,mimeType:'image/jpeg',preview:base64ToBlobUrl(outUrl)});
            };
            img.onerror=()=>{
                const base64=dataUrl.split(',')[1]||'';
                resolve({base64:base64,mimeType:file.type||'image/jpeg',preview:base64ToBlobUrl(dataUrl)});
            };
            img.src=dataUrl;
        });
    }

    function checkInput(){
        const hasText=!!(UI.textarea&&UI.textarea.value.trim().length>0);
        const hasImages=state.images.length>0;
        if(UI.sendBtn) UI.sendBtn.classList.toggle('active',hasText||hasImages);
        if(UI.enhanceBtn) UI.enhanceBtn.classList.toggle('active',hasText);
    }
    async function handleFiles(files){if(state.images.length+files.length>14){alert("最多14张");return}for(let file of files){if(!file.type.startsWith('image/'))continue;state.images.push(await compressImage(file))}renderPreviews();checkInput();if(UI.fileInput)UI.fileInput.value=''}function renderPreviews(){if(UI.previewArea){UI.previewArea.innerHTML='';if(state.images.length>0){UI.previewArea.classList.add('has-images');state.images.forEach((img,i)=>{const div=document.createElement('div');div.className='preview-item';div.style.backgroundImage=`url(${img.preview})`;div.innerHTML=`<div class="preview-close" onclick="state.images.splice(${i},1);renderPreviews();checkInput()">×</div>`;UI.previewArea.appendChild(div)})}else UI.previewArea.classList.remove('has-images')}}
