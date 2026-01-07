// 文件系统管理器 - 自动保存图片到本地目录
const FileSystemManager = {
    directoryHandle: null,
    isEnabled: false,

    // 检查浏览器是否支持 File System Access API
    isSupported() {
        return 'showDirectoryPicker' in window;
    },

    // 初始化
    init() {
        const autoSaveToggle = document.getElementById('auto-save-toggle');
        if (autoSaveToggle) {
            // 加载保存的设置
            const saved = localStorage.getItem('auto_save_enabled');
            if (saved === 'true') {
                // 检查目录句柄是否存在（刷新页面后会丢失）
                if (!this.directoryHandle) {
                    // 目录句柄丢失，重置状态并提示用户
                    console.warn('⚠️ 目录句柄丢失，需要重新选择目录');
                    localStorage.removeItem('auto_save_enabled');
                    autoSaveToggle.checked = false;
                    this.isEnabled = false;

                    // 隐藏目录路径显示
                    const pathDiv = document.getElementById('selected-dir-path');
                    if (pathDiv) pathDiv.style.display = 'none';

                    // 延迟显示提示，避免页面加载时过多提示
                    setTimeout(() => {
                        showToast('自动保存已重置，请重新选择保存目录', 'warning', 3000);
                    }, 1000);
                } else {
                    autoSaveToggle.checked = true;
                    this.isEnabled = true;
                }
            }

            // 监听开关变化
            autoSaveToggle.addEventListener('change', (e) => {
                this.isEnabled = e.target.checked;
                localStorage.setItem('auto_save_enabled', e.target.checked);

                if (e.target.checked && !this.directoryHandle) {
                    showToast('请先选择保存目录', 'warning');
                    e.target.checked = false;
                    this.isEnabled = false;
                    localStorage.removeItem('auto_save_enabled');
                }
            });
        }
    },

    // 选择目录
    async selectDirectory() {
        if (!this.isSupported()) {
            showToast('您的浏览器不支持此功能，请使用 Chrome 86+ 或 Edge 86+', 'error', 3000);
            return;
        }

        try {
            // 请求用户选择目录
            this.directoryHandle = await window.showDirectoryPicker({
                mode: 'readwrite'
            });

            // 显示选择的目录路径
            const pathDiv = document.getElementById('selected-dir-path');
            const pathText = document.getElementById('dir-path-text');
            if (pathDiv && pathText) {
                pathText.textContent = this.directoryHandle.name;
                pathDiv.style.display = 'block';
            }

            showToast('目录选择成功！', 'success');

            // 自动启用自动保存
            const autoSaveToggle = document.getElementById('auto-save-toggle');
            if (autoSaveToggle && !autoSaveToggle.checked) {
                autoSaveToggle.checked = true;
                this.isEnabled = true;
                localStorage.setItem('auto_save_enabled', 'true');
            }
        } catch (error) {
            if (error.name !== 'AbortError') {
                console.error('选择目录失败:', error);
                showToast('选择目录失败: ' + error.message, 'error');
            }
        }
    },

    // 保存图片到目录
    async saveImageToDirectory(base64Data, filename) {
        console.log('🔍 saveImageToDirectory 被调用');
        console.log('  - isEnabled:', this.isEnabled);
        console.log('  - directoryHandle:', this.directoryHandle);
        console.log('  - filename:', filename);

        if (!this.isEnabled || !this.directoryHandle) {
            console.log('❌ 保存条件不满足');
            return false;
        }

        try {
            console.log('📥 开始保存文件...');
            // 将 base64 转换为 Blob
            const response = await fetch(base64Data);
            const blob = await response.blob();
            console.log('✅ Blob 创建成功，大小:', blob.size);

            // 创建文件
            const fileHandle = await this.directoryHandle.getFileHandle(filename, { create: true });
            console.log('✅ 文件句柄创建成功');

            const writable = await fileHandle.createWritable();
            await writable.write(blob);
            await writable.close();
            console.log('✅ 文件写入成功！');

            return true;
        } catch (error) {
            console.error('❌ 保存文件失败:', error);
            showToast('保存文件失败: ' + error.message, 'error');
            return false;
        }
    }
};

function downloadImage(base64Data, filename) {
    console.log('📥 downloadImage 被调用');
    console.log('  - filename:', filename);
    console.log('  - FileSystemManager.isEnabled:', FileSystemManager.isEnabled);
    console.log('  - FileSystemManager.directoryHandle:', FileSystemManager.directoryHandle);

    // 优先尝试自动保存到本地目录
    if (FileSystemManager.isEnabled && FileSystemManager.directoryHandle) {
        console.log('✅ 满足自动保存条件，开始保存...');
        FileSystemManager.saveImageToDirectory(base64Data, filename).then(success => {
            if (success) {
                console.log('✅ 自动保存成功！');
                showToast('图片已保存到本地目录 ✓', 'success');
                return;
            }
            console.log('⚠️ 自动保存失败，使用下载方式');
            // 如果保存失败，继续使用下载方式
            proceedWithDownload();
        });
        return;
    }

    console.log('ℹ️ 使用传统下载方式');
    // 使用原有的下载方式
    proceedWithDownload();

    function proceedWithDownload() {
        const isIOS = /iPad|iPhone|iPod/.test(navigator.userAgent) && !window.MSStream;
        const isSafari = /^((?!chrome|android).)*safari/i.test(navigator.userAgent);
        const isLocalFile = window.location.protocol === 'file:';
    
    if (isIOS || isSafari || isLocalFile) {
        const newWindow = window.open();
        if (newWindow) {
            const tipText = isLocalFile 
                ? '<strong>💾 本地文件模式</strong>右键图片 → 另存为<br><small>或使用 HTTP 服务器运行以支持直接下载</small>'
                : '<strong>📱 保存方法</strong>长按图片 → 选择"存储图像"';
            
            newWindow.document.write(`
                <!DOCTYPE html>
                <html>
                <head>
                    <meta name="viewport" content="width=device-width, initial-scale=1.0">
                    <title>${filename}</title>
                    <style>
                        body { margin: 0; padding: 20px; background: #000; display: flex; flex-direction: column; align-items: center; justify-content: center; min-height: 100vh; }
                        img { max-width: 100%; height: auto; border-radius: 8px; box-shadow: 0 4px 20px rgba(255,255,255,0.1); }
                        .tip { color: #fff; margin-top: 20px; text-align: center; font-family: -apple-system, BlinkMacSystemFont, sans-serif; font-size: 14px; line-height: 1.6; }
                        .tip strong { display: block; margin-bottom: 8px; font-size: 16px; }
                        .tip small { display: block; margin-top: 8px; opacity: 0.7; font-size: 12px; }
                    </style>
                </head>
                <body>
                    <img src="${base64Data}" alt="${filename}">
                    <div class="tip">${tipText}</div>
                </body>
                </html>
            `);
            newWindow.document.close();
        } else {
            showToast('请允许弹出窗口以查看图片', 'warning', 3000);
        }
    } else {
        try {
            const link = document.createElement('a');
            link.href = base64Data;
            link.download = filename;
            document.body.appendChild(link);
            link.click();
            document.body.removeChild(link);
            showToast('下载成功', 'success');
        } catch (e) {
            console.error('Download failed:', e);
            const newWindow = window.open();
            if (newWindow) {
                newWindow.document.write(`
                    <!DOCTYPE html>
                    <html>
                    <head>
                        <meta name="viewport" content="width=device-width, initial-scale=1.0">
                        <title>${filename}</title>
                        <style>
                            body { margin: 0; padding: 20px; background: #000; display: flex; flex-direction: column; align-items: center; justify-content: center; min-height: 100vh; }
                            img { max-width: 100%; height: auto; border-radius: 8px; }
                            .tip { color: #fff; margin-top: 20px; text-align: center; font-family: -apple-system, BlinkMacSystemFont, sans-serif; font-size: 14px; line-height: 1.6; }
                            .tip strong { display: block; margin-bottom: 8px; font-size: 16px; color: #ff6b6b; }
                        </style>
                    </head>
                    <body>
                        <img src="${base64Data}" alt="${filename}">
                        <div class="tip">
                            <strong>⚠️ 下载失败</strong>
                            请右键图片选择"另存为"
                        </div>
                    </body>
                    </html>
                `);
                newWindow.document.close();
            } else {
                showToast('下载失败，请右键图片另存为', 'error', 3000);
            }
        }
    }
    } // 关闭 proceedWithDownload 函数
}
