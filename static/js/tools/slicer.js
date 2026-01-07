    // --- Slicer Logic ---
    const SlicerTool = {
        horizontalLines: [], verticalLines: [], isDragging: false, draggedLine: null, generatedBlobs: [],
        init() {
            this.modal = document.getElementById('slice-modal'); this.fileInput = document.getElementById('slice-file-input'); this.editorContainer = document.getElementById('slice-editor-container'); this.sourceImage = document.getElementById('slice-source-image'); this.emptyMsg = document.getElementById('slice-empty-msg'); this.overlay = document.getElementById('slice-overlay-canvas'); this.processBtn = document.getElementById('slice-process-btn'); this.clearBtn = document.getElementById('slice-clear-btn'); this.resultGrid = document.getElementById('slice-result-grid'); this.modeRadios = document.getElementsByName('slice-mode'); this.forceSquareCheckbox = document.getElementById('slice-force-square'); this.colorPickerBox = document.getElementById('slice-color-picker-box'); this.bgColorInput = document.getElementById('slice-bg-color'); this.downloadAllBtn = document.getElementById('slice-download-all-btn'); this.bindEvents();
        },
        openLocal() { this.modal.classList.add('active'); this.resetEditor(); this.sourceImage.src = ""; this.sourceImage.style.display = 'none'; this.emptyMsg.style.display = 'flex'; if(window.innerWidth > 768) setTimeout(() => this.fileInput.click(), 100); closeAllSidebars(); },
        open(imageUrl) { this.modal.classList.add('active'); this.emptyMsg.style.display = 'none'; this.sourceImage.crossOrigin = "Anonymous"; this.sourceImage.src = imageUrl; this.sourceImage.style.display = 'block'; this.sourceImage.onload = () => { this.resetEditor(); this.autoGrid(6, 4); }; this.sourceImage.onerror = () => { alert("无法加载该图片"); }; },
        close() { this.modal.classList.remove('active'); },
        resetEditor() { this.overlay.innerHTML = ''; this.horizontalLines = []; this.verticalLines = []; this.resultGrid.innerHTML = ''; this.processBtn.disabled = false; this.downloadAllBtn.disabled = true; this.generatedBlobs = []; this.emptyMsg.style.display = this.sourceImage.style.display === 'none' ? 'flex' : 'none'; },
        autoGrid(rows, cols) { for (let i = 1; i < rows; i++) this.addLine('h', (i / rows) * 100); for (let j = 1; j < cols; j++) this.addLine('v', (j / cols) * 100); },
        handleFile(files) { const file = files[0]; if (!file) return; const reader = new FileReader(); reader.onload = (event) => { this.emptyMsg.style.display = 'none'; this.sourceImage.style.display = 'block'; this.sourceImage.src = event.target.result; this.sourceImage.onload = () => this.resetEditor(); }; reader.readAsDataURL(file); },
        setMode(type, labelEl) { document.querySelectorAll('.radio-label').forEach(l => l.classList.remove('active')); labelEl.classList.add('active'); labelEl.querySelector('input').checked = true; },
        getPointerPos(e) { return (e.touches && e.touches.length > 0) ? { x: e.touches[0].clientX, y: e.touches[0].clientY } : { x: e.clientX, y: e.clientY }; },
        bindEvents() {
            this.forceSquareCheckbox.addEventListener('change', (e) => this.colorPickerBox.style.display = e.target.checked ? 'flex' : 'none');
            this.overlay.addEventListener('click', (e) => { if (this.isDragging || e.target !== this.overlay) return; const rect = this.overlay.getBoundingClientRect(); const pos = this.getPointerPos(e); const x = pos.x - rect.left; const y = pos.y - rect.top; let mode = 'horizontal'; for(const r of this.modeRadios) if(r.checked) mode = r.value; if (mode === 'horizontal') this.addLine('h', (y / rect.height) * 100); else this.addLine('v', (x / rect.width) * 100); });
            this.clearBtn.addEventListener('click', () => { this.overlay.innerHTML = ''; this.horizontalLines = []; this.verticalLines = []; });
            this.processBtn.addEventListener('click', () => this.process());
        },
        addLine(type, percent) {
            const line = document.createElement('div'); line.classList.add('split-line', type === 'h' ? 'horizontal' : 'vertical'); const delBtn = document.createElement('div'); delBtn.className = 'delete-btn-line'; delBtn.innerText = '×';
            if(type === 'h') { line.style.top = percent + '%'; delBtn.style.right = '0'; delBtn.style.top = '-12px'; this.horizontalLines.push({ percent: percent, element: line }); } else { line.style.left = percent + '%'; delBtn.style.bottom = '-12px'; delBtn.style.left = '-12px'; this.verticalLines.push({ percent: percent, element: line }); }
            delBtn.addEventListener('click', (e) => { e.stopPropagation(); this.removeLine(line, type); }); delBtn.addEventListener('touchend', (e) => { e.preventDefault(); e.stopPropagation(); this.removeLine(line, type); });
            line.appendChild(delBtn); line.addEventListener('mousedown', (e) => this.startDrag(e, line, type)); line.addEventListener('touchstart', (e) => this.startDrag(e, line, type)); this.overlay.appendChild(line);
        },
        removeLine(element, type) { element.remove(); if (type === 'h') this.horizontalLines = this.horizontalLines.filter(l => l.element !== element); else this.verticalLines = this.verticalLines.filter(l => l.element !== element); },
        startDrag(e, element, type) {
            e.stopPropagation(); this.isDragging = true; this.draggedLine = { element, type };
            const moveHandler = (ev) => this.onDrag(ev);
            const upHandler = () => { this.isDragging = false; this.draggedLine = null; document.removeEventListener('mousemove', moveHandler); document.removeEventListener('mouseup', upHandler); document.removeEventListener('touchmove', moveHandler); document.removeEventListener('touchend', upHandler); setTimeout(() => { this.isDragging = false }, 50); };
            document.addEventListener('mousemove', moveHandler); document.addEventListener('mouseup', upHandler); document.addEventListener('touchmove', moveHandler, { passive: false }); document.addEventListener('touchend', upHandler);
        },
        onDrag(e) {
            if (!this.isDragging || !this.draggedLine) return; if (e.type === 'touchmove') e.preventDefault();
            const rect = this.overlay.getBoundingClientRect(); const pos = this.getPointerPos(e); let percent;
            if (this.draggedLine.type === 'h') { let y = Math.max(0, Math.min(pos.y - rect.top, rect.height)); percent = (y / rect.height) * 100; this.draggedLine.element.style.top = percent + '%'; const lineObj = this.horizontalLines.find(l => l.element === this.draggedLine.element); if(lineObj) lineObj.percent = percent; }
            else { let x = Math.max(0, Math.min(pos.x - rect.left, rect.width)); percent = (x / rect.width) * 100; this.draggedLine.element.style.left = percent + '%'; const lineObj = this.verticalLines.find(l => l.element === this.draggedLine.element); if(lineObj) lineObj.percent = percent; }
        },
        async process() {
            this.resultGrid.innerHTML = '<div style="width:100%;text-align:center;padding:20px;color:#666;">⚡ 正在处理...</div>';
            this.generatedBlobs = [];
            this.processBtn.disabled = true;
            this.downloadAllBtn.disabled = true;

            const imgRealWidth = this.sourceImage.naturalWidth;
            const imgRealHeight = this.sourceImage.naturalHeight;
            const isForceSquare = this.forceSquareCheckbox.checked;
            const fillColor = this.bgColorInput.value;

            let hCuts = this.horizontalLines.map(l => (l.percent / 100) * imgRealHeight);
            hCuts.push(0, imgRealHeight);
            hCuts.sort((a, b) => a - b);

            let vCuts = this.verticalLines.map(l => (l.percent / 100) * imgRealWidth);
            vCuts.push(0, imgRealWidth);
            vCuts.sort((a, b) => a - b);

            await new Promise(resolve => setTimeout(resolve, 50));

            this.resultGrid.innerHTML = '';
            const fragment = document.createDocumentFragment();
            const blobPromises = [];

            for (let i = 0; i < hCuts.length - 1; i++) {
                for (let j = 0; j < vCuts.length - 1; j++) {
                    const srcX = vCuts[j];
                    const srcY = hCuts[i];
                    const srcW = vCuts[j+1] - vCuts[j];
                    const srcH = hCuts[i+1] - hCuts[i];

                    if (srcW < 1 || srcH < 1) continue;

                    const canvas = document.createElement('canvas');
                    const ctx = canvas.getContext('2d', { alpha: true });

                    // 使用2x分辨率提高清晰度
                    const scale = 2;

                    if (isForceSquare) {
                        const maxDim = Math.max(srcW, srcH);
                        canvas.width = maxDim * scale;
                        canvas.height = maxDim * scale;

                        ctx.fillStyle = fillColor;
                        ctx.fillRect(0, 0, canvas.width, canvas.height);

                        ctx.imageSmoothingEnabled = true;
                        ctx.imageSmoothingQuality = 'high';

                        const offsetX = (maxDim - srcW) / 2;
                        const offsetY = (maxDim - srcH) / 2;
                        ctx.drawImage(
                            this.sourceImage,
                            srcX, srcY, srcW, srcH,
                            offsetX * scale, offsetY * scale, srcW * scale, srcH * scale
                        );
                    } else {
                        canvas.width = srcW * scale;
                        canvas.height = srcH * scale;

                        ctx.imageSmoothingEnabled = true;
                        ctx.imageSmoothingQuality = 'high';

                        ctx.drawImage(
                            this.sourceImage,
                            srcX, srcY, srcW, srcH,
                            0, 0, srcW * scale, srcH * scale
                        );
                    }

                    const itemName = `slice_${i+1}_${j+1}.png`;
                    const row = i;
                    const col = j;

                    const blobPromise = new Promise((resolve, reject) => {
                        try {
                            canvas.toBlob(blob => {
                                if (!blob) {
                                    reject(new Error('Failed to create blob'));
                                    return;
                                }

                                const blobUrl = BlobManager.create(blob);
                                this.generatedBlobs.push({ blob: blob, name: itemName });

                                const card = document.createElement('div');
                                card.className = 'slice-card';

                                const img = document.createElement('img');
                                img.src = blobUrl;
                                img.className = 'slice-img-result';

                                const info = document.createElement('div');
                                info.className = 'slice-info';
                                info.innerText = `${Math.round(canvas.width / scale)} x ${Math.round(canvas.height / scale)} (${scale}x)`;

                                card.onclick = () => {
                                    const a = document.createElement('a');
                                    a.href = blobUrl;
                                    a.download = itemName;
                                    a.click();
                                };

                                card.appendChild(img);
                                card.appendChild(info);
                                fragment.appendChild(card);

                                resolve();
                            }, 'image/png', 1.0);
                        } catch(e) {
                            reject(e);
                        }
                    });

                    blobPromises.push(blobPromise);
                }
            }

            try {
                await Promise.all(blobPromises);
                this.resultGrid.appendChild(fragment);
                this.downloadAllBtn.disabled = false;
                this.processBtn.disabled = false;
                showToast(`成功生成 ${this.generatedBlobs.length} 个切片`, 'success');
            } catch(e) {
                console.error('切片处理失败:', e);
                this.resultGrid.innerHTML = '<div style="width:100%;text-align:center;padding:20px;color:#d93025;">⚠️ 处理失败: ' + e.message + '</div>';
                this.processBtn.disabled = false;
            }
        },
        async downloadAll() { if(this.generatedBlobs.length === 0) return; if(typeof JSZip === 'undefined') { try { await loadJSZip(); } catch(e) { alert('JSZip 加载失败'); return; } } const zip = new JSZip(); const folder = zip.folder("slices"); this.generatedBlobs.forEach(item => folder.file(item.name, item.blob)); const content = await zip.generateAsync({type:"blob"}); const a = document.createElement('a'); a.href = URL.createObjectURL(content); a.download = `slices_${Date.now()}.zip`; a.click(); }
    };
