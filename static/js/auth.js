function showError(message) {
    const errorEl = document.getElementById('error-message');
    if (errorEl) {
        errorEl.textContent = message;
        errorEl.style.display = 'block';
    }
}

function hideError() {
    const errorEl = document.getElementById('error-message');
    if (errorEl) {
        errorEl.style.display = 'none';
    }
}

function setLoading(loading) {
    const btn = document.getElementById('submit-btn');
    const btnText = btn.querySelector('.btn-text');
    const btnLoading = btn.querySelector('.btn-loading');

    if (loading) {
        btn.disabled = true;
        btnText.style.display = 'none';
        btnLoading.style.display = 'inline';
    } else {
        btn.disabled = false;
        btnText.style.display = 'inline';
        btnLoading.style.display = 'none';
    }
}

function validatePasswordComplexity(password) {
    const hasUpper = /[A-Z]/.test(password);
    const hasLower = /[a-z]/.test(password);
    const hasDigit = /[0-9]/.test(password);
    return hasUpper && hasLower && hasDigit;
}

async function handleLogin(email, password, rememberMe = false) {
    hideError();
    setLoading(true);

    try {
        const resp = await fetch('/api/auth/login', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ email, password, remember_me: rememberMe })
        });

        const data = await resp.json();

        if (resp.ok) {
            if (data.email_verified === false) {
                window.location.href = '/verify-pending';
            } else {
                window.location.href = '/';
            }
        } else {
            showError(data.error || '登录失败');
        }
    } catch (e) {
        showError('网络错误，请重试');
    } finally {
        setLoading(false);
    }
}

async function handleRegister(email, password) {
    hideError();
    setLoading(true);

    try {
        const resp = await fetch('/api/auth/register', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ email, password })
        });

        const data = await resp.json();

        if (resp.ok) {
            window.location.href = '/verify-pending';
        } else {
            showError(data.error || '注册失败');
        }
    } catch (e) {
        showError('网络错误，请重试');
    } finally {
        setLoading(false);
    }
}

async function handleLogout() {
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

async function checkAuth() {
    try {
        const resp = await fetch('/api/auth/me');
        if (resp.ok) {
            const user = await resp.json();
            window.currentUser = user;
            return true;
        }
        return false;
    } catch (e) {
        return false;
    }
}

function toggleTheme() {
    const current = document.documentElement.getAttribute('data-theme') || 'light';
    const next = current === 'dark' ? 'light' : 'dark';
    document.documentElement.setAttribute('data-theme', next);
    localStorage.setItem('theme', next);
}
