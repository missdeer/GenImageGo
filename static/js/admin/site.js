(function() {
    let configs = [];
    let isSuperAdmin = false;

    async function init() {
        try {
            const resp = await fetch('/api/auth/me');
            if (!resp.ok) {
                window.location.href = '/login?redirect=' + encodeURIComponent(window.location.pathname);
                return;
            }
            const user = await resp.json();

            if (!user.is_super_admin) {
                window.location.href = '/';
                return;
            }

            isSuperAdmin = true;
            await loadConfigs();
        } catch (e) {
            window.location.href = '/login?redirect=' + encodeURIComponent(window.location.pathname);
        }
    }

    async function loadConfigs() {
        const loading = document.getElementById('loading');
        const errorMsg = document.getElementById('error-message');
        const configForm = document.getElementById('config-form');

        try {
            const response = await fetch('/api/admin/site/config', {
                method: 'GET',
                credentials: 'include'
            });

            if (!response.ok) {
                if (response.status === 403) {
                    loading.style.display = 'none';
                    document.getElementById('no-permission').style.display = 'block';
                    return;
                }
                throw new Error('加载配置失败');
            }

            const data = await response.json();
            configs = data.configs || [];

            renderConfigs();
            loading.style.display = 'none';
            configForm.style.display = 'block';
        } catch (err) {
            loading.style.display = 'none';
            errorMsg.textContent = err.message;
            errorMsg.style.display = 'block';
        }
    }

    function renderConfigs() {
        const container = document.getElementById('config-items');
        container.innerHTML = '';

        configs.forEach(config => {
            const item = document.createElement('div');
            item.className = 'config-item';

            const label = document.createElement('label');
            label.textContent = config.label;
            label.setAttribute('for', 'config-' + config.key);

            const input = document.createElement('input');
            input.type = config.type === 'int' ? 'number' : 'text';
            input.id = 'config-' + config.key;
            input.value = config.value;
            input.dataset.key = config.key;
            if (config.type === 'int') {
                input.min = '0';
                input.max = '1000';
            }

            const hint = document.createElement('span');
            hint.className = 'config-hint';
            hint.textContent = 'key: ' + config.key;

            item.appendChild(label);
            item.appendChild(input);
            item.appendChild(hint);
            container.appendChild(item);
        });
    }

    window.saveConfigs = async function() {
        const saveBtn = document.getElementById('save-btn');
        const saveStatus = document.getElementById('save-status');

        saveBtn.disabled = true;
        saveStatus.textContent = '保存中...';
        saveStatus.className = 'save-status';

        const updates = [];
        configs.forEach(config => {
            const input = document.getElementById('config-' + config.key);
            if (input) {
                updates.push({
                    key: config.key,
                    value: input.value
                });
            }
        });

        try {
            const response = await fetch('/api/admin/site/config', {
                method: 'POST',
                credentials: 'include',
                headers: {
                    'Content-Type': 'application/json',
                    'X-CSRF-Token': getCSRFToken()
                },
                body: JSON.stringify({ configs: updates })
            });

            if (!response.ok) {
                const data = await response.json();
                throw new Error(data.error || data.error?.message || '保存失败');
            }

            saveStatus.textContent = '保存成功';
            saveStatus.className = 'save-status success';

            await loadConfigs();
        } catch (err) {
            saveStatus.textContent = err.message;
            saveStatus.className = 'save-status error';
        } finally {
            saveBtn.disabled = false;
            setTimeout(() => {
                saveStatus.textContent = '';
            }, 3000);
        }
    };

    document.addEventListener('DOMContentLoaded', init);
})();
