function escapeHtml(text) { return text.replace(/[&<>"']/g, m => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#039;' }[m])); }

// 保存原生 fetch，绕过扩展拦截（修复 ERR_SSL_PROTOCOL_ERROR）
const nativeFetch = window.fetch.bind(window);

// 错误处理工具
const ErrorHandler = {
    show(title, message, actions = []) {
        showToast(`${title}: ${message}`, 'error', 4000);
        console.error(`[Error] ${title}:`, message);
    },
    handleAPIError(error, context = '操作') {
        let message = '未知错误';
        if (error.message) {
            message = error.message;
        } else if (typeof error === 'string') {
            message = error;
        }

        // 根据错误类型提供友好提示
        if (message.includes('fetch') || message.includes('network')) {
            this.show(`${context}失败`, '网络连接失败，请检查网络后重试');
        } else if (message.includes('401') || message.includes('403')) {
            this.show(`${context}失败`, 'API密钥无效或已过期，请检查配置');
        } else if (message.includes('429')) {
            this.show(`${context}失败`, '请求过于频繁，请稍后再试');
        } else if (message.includes('500') || message.includes('502') || message.includes('503')) {
            this.show(`${context}失败`, '服务器错误，请稍后重试');
        } else {
            this.show(`${context}失败`, message);
        }
    }
};

async function loadJSZip() { if (window.JSZip) return; await new Promise((r, j) => { const s = document.createElement('script'); s.src = 'https://cdnjs.cloudflare.com/ajax/libs/jszip/3.10.1/jszip.min.js'; s.onload = r; s.onerror = j; document.head.appendChild(s); }); }

const BlobManager = {
    urls: [],
    create(blob) { const url = URL.createObjectURL(blob); this.urls.push(url); return url; },
    cleanup() { this.urls.forEach(url => URL.revokeObjectURL(url)); this.urls = []; }
};

function base64ToBlobUrl(base64Data){try{const arr=base64Data.split(',');const mime=arr[0].match(/:(.*?);/)[1];const bstr=atob(arr[1]);let n=bstr.length;const u8arr=new Uint8Array(n);while(n--){u8arr[n]=bstr.charCodeAt(n)}return BlobManager.create(new Blob([u8arr],{type:mime}))}catch(e){console.error(e);return''}}

function copyText(btn){navigator.clipboard.writeText(btn.getAttribute('data-text')).then(()=>{const original=btn.innerHTML;btn.innerHTML='<span>已复制</span>';setTimeout(()=>btn.innerHTML=original,1500)})}

function openLightbox(src){document.getElementById('lightbox-image').src=src;document.getElementById('lightbox').classList.add('active')}
function closeLightbox(){document.getElementById('lightbox').classList.remove('active');setTimeout(()=>document.getElementById('lightbox-image').src='',200)}

const leftSidebar=document.getElementById('left-sidebar');const rightSidebar=document.getElementById('right-sidebar');const overlay=document.getElementById('overlay');
function toggleLeftSidebar(){leftSidebar.classList.toggle('open');overlay.classList.toggle('active');rightSidebar.classList.remove('open')}
function toggleSettings(){rightSidebar.classList.toggle('open');overlay.classList.toggle('active');leftSidebar.classList.remove('open')}
function closeAllSidebars(){leftSidebar.classList.remove('open');rightSidebar.classList.remove('open');overlay.classList.remove('active')}

function toggleLeftSidebarDesktop(){
    const isCollapsed = leftSidebar.classList.toggle('collapsed');
    const btn = document.getElementById('left-sidebar-toggle');
    if(btn) btn.classList.toggle('active', isCollapsed);
    localStorage.setItem('left_sidebar_collapsed', isCollapsed);
}

function toggleRightSidebarDesktop(){
    const isCollapsed = rightSidebar.classList.toggle('collapsed');
    const btn = document.getElementById('right-sidebar-toggle');
    if(btn) btn.classList.toggle('active', isCollapsed);
    localStorage.setItem('right_sidebar_collapsed', isCollapsed);
}

function restoreSidebarState(){
    if(localStorage.getItem('left_sidebar_collapsed') === 'true'){
        leftSidebar.classList.add('collapsed');
        const btn = document.getElementById('left-sidebar-toggle');
        if(btn) btn.classList.add('active');
    }
    if(localStorage.getItem('right_sidebar_collapsed') === 'true'){
        rightSidebar.classList.add('collapsed');
        const btn = document.getElementById('right-sidebar-toggle');
        if(btn) btn.classList.add('active');
    }
}

document.addEventListener('DOMContentLoaded', () => setTimeout(restoreSidebarState, 100));
