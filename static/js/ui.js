// ===== 增强的用户反馈系统 =====

// Toast 提示（增强版）
function showToast(msg, type = 'default', duration = 2000) {
    const t = document.getElementById('toast');
    t.textContent = msg;
    t.className = 'toast-msg';
    if (type !== 'default') t.classList.add(type);
    t.style.display = 'block';
    setTimeout(() => t.style.display = 'none', duration);
}

// 全局加载遮罩
const LoadingManager = {
    show(text = '加载中...') {
        const loading = document.getElementById('global-loading');
        const loadingText = document.getElementById('global-loading-text');
        if (loadingText) loadingText.textContent = text;
        if (loading) loading.classList.add('active');
    },
    hide() {
        const loading = document.getElementById('global-loading');
        if (loading) loading.classList.remove('active');
    },
    updateText(text) {
        const loadingText = document.getElementById('global-loading-text');
        if (loadingText) loadingText.textContent = text;
    }
};

// 进度条管理
const ProgressBar = {
    show() {
        const container = document.getElementById('progress-bar-container');
        if (container) container.classList.add('active');
    },
    hide() {
        const container = document.getElementById('progress-bar-container');
        const bar = document.getElementById('progress-bar');
        if (container) container.classList.remove('active');
        if (bar) bar.style.width = '0%';
    },
    setProgress(percent) {
        const bar = document.getElementById('progress-bar');
        if (bar) bar.style.width = `${Math.min(100, Math.max(0, percent))}%`;
    }
};

// 智能进度条管理器（用于单图生成）
const SmartProgressBar = {
    timers: new Map(),

    // 根据分辨率估算生成时间（秒）
    estimateTime(resolution, hasRefImages = false) {
        const baseTime = {
            '1024x1024': 20,
            '1K': 20,
            '2048x2048': 45,
            '2K': 45,
            '4096x4096': 90,
            '4K': 90
        };
        let time = baseTime[resolution] || 30;
        // 如果有参考图，增加 30% 时间
        if (hasRefImages) time *= 1.3;
        return time;
    },

    // 缓动函数：开始快，后面慢
    easeOutCubic(t) {
        return 1 - Math.pow(1 - t, 3);
    },

    // 启动进度条
    start(elementId, resolution, hasRefImages = false) {
        this.stop(elementId); // 先清除旧的

        const totalTime = this.estimateTime(resolution, hasRefImages);
        const startTime = Date.now();
        const maxProgress = 95; // 最多到 95%，等待实际完成

        const updateProgress = () => {
            const elapsed = (Date.now() - startTime) / 1000;
            const rawProgress = Math.min(elapsed / totalTime, 1);
            const easedProgress = this.easeOutCubic(rawProgress);
            const percent = Math.floor(easedProgress * maxProgress);

            const barEl = document.getElementById(elementId);
            const textEl = document.getElementById(elementId + '-text');

            if (barEl) {
                barEl.style.width = percent + '%';
            }
            if (textEl) {
                textEl.textContent = percent + '%';
            }

            if (percent < maxProgress) {
                const timer = setTimeout(updateProgress, 100);
                this.timers.set(elementId, timer);
            }
        };

        updateProgress();
    },

    // 完成进度条（跳到 100%）
    complete(elementId, callback) {
        this.stop(elementId);

        const barEl = document.getElementById(elementId);
        const textEl = document.getElementById(elementId + '-text');

        if (barEl) barEl.style.width = '100%';
        if (textEl) textEl.textContent = '100%';

        // 短暂显示 100% 后执行回调
        if (callback) {
            setTimeout(callback, 300);
        }
    },

    // 停止进度条
    stop(elementId) {
        const timer = this.timers.get(elementId);
        if (timer) {
            clearTimeout(timer);
            this.timers.delete(elementId);
        }
    },

    // 创建进度条 HTML
    createHTML(id) {
        return `
            <div style="margin: 20px 0; padding: 12px 16px; background: #f8f9fa; border-radius: 8px; border: 1px solid #e8eaed;">
                <div style="display: flex; align-items: center; justify-content: space-between; margin-bottom: 8px;">
                    <span style="font-size: 13px; color: #5f6368; font-weight: 500;">🎨 图片生成中</span>
                    <span id="${id}-text" style="font-size: 13px; color: #1967d2; font-weight: 600;">0%</span>
                </div>
                <div style="width: 100%; height: 6px; background: #e8eaed; border-radius: 3px; overflow: hidden;">
                    <div id="${id}" style="width: 0%; height: 100%; background: linear-gradient(90deg, #1967d2, #4285f4); border-radius: 3px; transition: width 0.3s ease;"></div>
                </div>
                <div style="font-size: 11px; color: #80868b; margin-top: 6px;">根据分辨率预估时间，实际可能有偏差</div>
            </div>
        `;
    }
};

// 主题切换功能
function initTheme() {
    const savedTheme = localStorage.getItem('theme') || 'light';
    document.documentElement.setAttribute('data-theme', savedTheme);
}

function toggleTheme() {
    const currentTheme = document.documentElement.getAttribute('data-theme') || 'light';
    const newTheme = currentTheme === 'light' ? 'dark' : 'light';
    document.documentElement.setAttribute('data-theme', newTheme);
    localStorage.setItem('theme', newTheme);
    showToast(newTheme === 'dark' ? '已切换到暗黑模式 🌙' : '已切换到明亮模式 ☀️', 'success');
}

// 页面加载时初始化主题
initTheme();

// ===== 用户菜单 =====

function toggleUserMenu(event) {
    if (event) {
        event.stopPropagation();
    }
    const container = event?.currentTarget?.closest('.user-menu-container') ||
        document.querySelector('.user-menu-container');
    if (!container) return;

    const dropdown = container.querySelector('.user-dropdown');
    if (!dropdown) return;

    const isActive = dropdown.classList.contains('active');
    closeUserMenu();
    if (!isActive) {
        // 更新用户邮箱显示
        const emailEl = container.querySelector('.user-dropdown-email');
        if (emailEl && window.currentUser) {
            emailEl.textContent = window.currentUser.email;
        }
        dropdown.classList.add('active');
        // 点击其他地方关闭菜单
        setTimeout(() => {
            document.addEventListener('click', closeUserMenuOnClickOutside);
        }, 0);
    }
}

function closeUserMenu() {
    document.querySelectorAll('.user-dropdown.active').forEach((dropdown) => {
        dropdown.classList.remove('active');
    });
    document.removeEventListener('click', closeUserMenuOnClickOutside);
}

function closeUserMenuOnClickOutside(e) {
    const openDropdown = document.querySelector('.user-dropdown.active');
    if (!openDropdown) return;
    const container = openDropdown.closest('.user-menu-container');
    if (container && !container.contains(e.target)) {
        closeUserMenu();
    }
}

function showUserProfile() {
    closeUserMenu();
    window.location.href = '/profile';
}

async function handleLogout() {
    closeUserMenu();
    try {
        await fetch('/api/auth/logout', { 
            method: 'POST',
            headers: { 'X-CSRF-Token': getCSRFToken() }
        });
        window.location.href = '/login';
    } catch (e) {
        console.error('Logout failed:', e);
        window.location.href = '/login';
    }
}
