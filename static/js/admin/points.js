(function() {
    let userInfo = null;
    let permissions = null;
    let managedOrgs = [];
    let isSuperAdmin = false;
    let isOrgAdmin = false;

    const reasonTextMap = {
        'daily_login': '每日登录',
        'referral_bonus': '推荐奖励',
        'referred_bonus': '被推荐奖励',
        'image_generation': '生成图片',
        'enhance_prompt': '提示词优化',
        'refund': '积分退还',
        'admin_grant': '管理员充值',
        'org_allocation': '组织划拨',
        'org_initial': '组织初始积分',
        'org_adjust': '组织积分调整',
        'org_allocation_out': '组织划拨支出'
    };

    const state = {
        my: { page: 1, totalPages: 1 },
        user: { page: 1, totalPages: 1, selectedUserId: null },
        org: { page: 1, totalPages: 1, selectedOrgId: null }
    };

    async function init() {
        try {
            const resp = await fetch('/api/auth/me');
            if (!resp.ok) {
                window.location.href = '/login';
                return;
            }
            userInfo = await resp.json();
            isSuperAdmin = userInfo.is_super_admin;
            isOrgAdmin = userInfo.can_manage_users && !isSuperAdmin;
            managedOrgs = userInfo.managed_organizations || [];

            document.getElementById('loading').style.display = 'none';
            document.getElementById('main-content').style.display = 'block';

            setupTabs();
            setupReasonFilters();
            setupEventListeners();

            if (isSuperAdmin || isOrgAdmin) {
                loadOrganizations();
            }
            if (isSuperAdmin) {
                loadUsers('');
            }

            loadMyHistory();
        } catch (e) {
            showError('加载失败: ' + e.message);
        }
    }

    function setupTabs() {
        const container = document.getElementById('tabs-container');
        container.innerHTML = '';

        const myTab = createTabBtn('my', '我的积分');
        container.appendChild(myTab);

        if (isSuperAdmin) {
            const userTab = createTabBtn('user', '用户积分');
            const orgTab = createTabBtn('org', '组织积分');
            container.appendChild(userTab);
            container.appendChild(orgTab);
        } else if (isOrgAdmin && managedOrgs.length > 0) {
            const orgTab = createTabBtn('org', '组织积分');
            container.appendChild(orgTab);
        }

        switchTab('my');
    }

    function createTabBtn(id, label) {
        const btn = document.createElement('button');
        btn.className = 'tab-btn';
        btn.dataset.tab = id;
        btn.textContent = label;
        btn.addEventListener('click', () => switchTab(id));
        return btn;
    }

    function switchTab(tabId) {
        document.querySelectorAll('.tab-btn').forEach(btn => {
            btn.classList.toggle('active', btn.dataset.tab === tabId);
        });
        document.querySelectorAll('.tab-content').forEach(content => {
            content.classList.toggle('active', content.id === 'tab-' + tabId);
        });
        if (tabId === 'user' && !state.user.selectedUserId) {
            document.getElementById('user-table').style.display = 'none';
            document.getElementById('user-pagination').style.display = 'none';
            document.getElementById('user-empty').style.display = 'block';
        }
        if (tabId === 'org' && !state.org.selectedOrgId) {
            document.getElementById('org-table').style.display = 'none';
            document.getElementById('org-pagination').style.display = 'none';
            document.getElementById('org-empty').style.display = 'block';
        }
    }

    function setupReasonFilters() {
        const userReasons = [
            'daily_login', 'referral_bonus', 'referred_bonus',
            'image_generation', 'enhance_prompt', 'refund',
            'admin_grant', 'org_allocation'
        ];
        const orgReasons = [
            'org_initial', 'org_adjust', 'org_allocation_out'
        ];

        populateReasonSelect('my-reason-filter', userReasons);
        populateReasonSelect('user-reason-filter', userReasons);
        populateReasonSelect('org-reason-filter', orgReasons);
    }

    function populateReasonSelect(selectId, reasons) {
        const select = document.getElementById(selectId);
        reasons.forEach(reason => {
            const option = document.createElement('option');
            option.value = reason;
            option.textContent = reasonTextMap[reason] || reason;
            select.appendChild(option);
        });
    }

    function setupEventListeners() {
        document.getElementById('my-search-btn').addEventListener('click', () => {
            state.my.page = 1;
            loadMyHistory();
        });
        document.getElementById('my-prev-btn').addEventListener('click', () => {
            if (state.my.page > 1) {
                state.my.page--;
                loadMyHistory();
            }
        });
        document.getElementById('my-next-btn').addEventListener('click', () => {
            if (state.my.page < state.my.totalPages) {
                state.my.page++;
                loadMyHistory();
            }
        });

        if (isSuperAdmin) {
            document.getElementById('user-search-keyword').addEventListener('input', debounce((e) => {
                loadUsers(e.target.value.trim());
            }, 300));
            document.getElementById('user-search-btn').addEventListener('click', () => {
                state.user.page = 1;
                loadUserHistory();
            });
            document.getElementById('user-select').addEventListener('change', (e) => {
                state.user.selectedUserId = e.target.value ? parseInt(e.target.value) : null;
                state.user.page = 1;
                if (state.user.selectedUserId) {
                    loadUserHistory();
                } else {
                    document.getElementById('user-table').style.display = 'none';
                    document.getElementById('user-pagination').style.display = 'none';
                    document.getElementById('user-empty').style.display = 'block';
                }
            });
            document.getElementById('user-prev-btn').addEventListener('click', () => {
                if (state.user.page > 1) {
                    state.user.page--;
                    loadUserHistory();
                }
            });
            document.getElementById('user-next-btn').addEventListener('click', () => {
                if (state.user.page < state.user.totalPages) {
                    state.user.page++;
                    loadUserHistory();
                }
            });
        }

        if (isSuperAdmin || isOrgAdmin) {
            document.getElementById('org-search-btn').addEventListener('click', () => {
                state.org.page = 1;
                loadOrgHistory();
            });
            document.getElementById('org-select').addEventListener('change', (e) => {
                state.org.selectedOrgId = e.target.value ? parseInt(e.target.value) : null;
                state.org.page = 1;
                if (state.org.selectedOrgId) {
                    loadOrgHistory();
                } else {
                    document.getElementById('org-table').style.display = 'none';
                    document.getElementById('org-pagination').style.display = 'none';
                    document.getElementById('org-empty').style.display = 'block';
                }
            });
            document.getElementById('org-prev-btn').addEventListener('click', () => {
                if (state.org.page > 1) {
                    state.org.page--;
                    loadOrgHistory();
                }
            });
            document.getElementById('org-next-btn').addEventListener('click', () => {
                if (state.org.page < state.org.totalPages) {
                    state.org.page++;
                    loadOrgHistory();
                }
            });
        }
    }

    async function loadMyHistory() {
        clearError();
        const reason = document.getElementById('my-reason-filter').value;
        const startDate = document.getElementById('my-start-date').value;
        const endDate = document.getElementById('my-end-date').value;

        let url = `/api/user/points/history?page=${state.my.page}&page_size=20`;
        if (reason) url += `&reason=${encodeURIComponent(reason)}`;
        if (startDate) url += `&start_date=${startDate}`;
        if (endDate) url += `&end_date=${endDate}`;

        try {
            const resp = await fetch(url);
            if (!resp.ok) {
                const data = await resp.json();
                throw new Error(data.error || '请求失败');
            }
            const data = await resp.json();
            renderTransactions('my', data);
        } catch (e) {
            showError(e.message);
        }
    }

    async function loadUserHistory() {
        if (!state.user.selectedUserId) return;
        clearError();

        const reason = document.getElementById('user-reason-filter').value;
        const startDate = document.getElementById('user-start-date').value;
        const endDate = document.getElementById('user-end-date').value;

        let url = `/api/admin/user/points/history?user_id=${state.user.selectedUserId}&page=${state.user.page}&page_size=20`;
        if (reason) url += `&reason=${encodeURIComponent(reason)}`;
        if (startDate) url += `&start_date=${startDate}`;
        if (endDate) url += `&end_date=${endDate}`;

        try {
            const resp = await fetch(url);
            if (!resp.ok) {
                const data = await resp.json();
                throw new Error(data.error || '请求失败');
            }
            const data = await resp.json();
            document.getElementById('user-empty').style.display = 'none';
            renderTransactions('user', data);
        } catch (e) {
            showError(e.message);
        }
    }

    async function loadOrgHistory() {
        if (!state.org.selectedOrgId) return;
        clearError();

        const reason = document.getElementById('org-reason-filter').value;
        const startDate = document.getElementById('org-start-date').value;
        const endDate = document.getElementById('org-end-date').value;

        let url = `/api/admin/org/points/history?organization_id=${state.org.selectedOrgId}&page=${state.org.page}&page_size=20`;
        if (reason) url += `&reason=${encodeURIComponent(reason)}`;
        if (startDate) url += `&start_date=${startDate}`;
        if (endDate) url += `&end_date=${endDate}`;

        try {
            const resp = await fetch(url);
            if (!resp.ok) {
                const data = await resp.json();
                throw new Error(data.error || '请求失败');
            }
            const data = await resp.json();
            document.getElementById('org-empty').style.display = 'none';
            renderTransactions('org', data);
        } catch (e) {
            showError(e.message);
        }
    }

    function renderTransactions(prefix, data) {
        const tbody = document.getElementById(prefix + '-tbody');
        tbody.innerHTML = '';

        if (!data.transactions || data.transactions.length === 0) {
            tbody.innerHTML = '<tr><td colspan="5" class="empty-message">暂无记录</td></tr>';
            document.getElementById(prefix + '-table').style.display = 'table';
            document.getElementById(prefix + '-pagination').style.display = 'none';
            return;
        }

        data.transactions.forEach(t => {
            const tr = document.createElement('tr');

            const timeTd = document.createElement('td');
            timeTd.textContent = formatDateTime(t.created_at);
            tr.appendChild(timeTd);

            const reasonTd = document.createElement('td');
            const badge = document.createElement('span');
            badge.className = 'reason-badge';
            badge.textContent = t.reason_text || reasonTextMap[t.reason] || t.reason;
            reasonTd.appendChild(badge);
            tr.appendChild(reasonTd);

            const amountTd = document.createElement('td');
            amountTd.className = t.amount >= 0 ? 'amount-positive' : 'amount-negative';
            amountTd.textContent = (t.amount >= 0 ? '+' : '') + t.amount;
            tr.appendChild(amountTd);

            const balanceTd = document.createElement('td');
            balanceTd.textContent = t.balance_after;
            tr.appendChild(balanceTd);

            const descTd = document.createElement('td');
            descTd.textContent = t.description || '-';
            tr.appendChild(descTd);

            tbody.appendChild(tr);
        });

        document.getElementById(prefix + '-table').style.display = 'table';

        const page = Number(data.page) || 1;
        const totalPages = Number(data.total_pages) || 1;

        state[prefix].totalPages = totalPages;
        state[prefix].page = page;
        document.getElementById(prefix + '-page-info').textContent = `第 ${page} 页 / 共 ${totalPages} 页`;
        document.getElementById(prefix + '-prev-btn').disabled = page <= 1;
        document.getElementById(prefix + '-next-btn').disabled = page >= totalPages;
        document.getElementById(prefix + '-pagination').style.display = 'flex';
    }

    async function loadOrganizations() {
        const select = document.getElementById('org-select');
        select.innerHTML = '<option value="">选择组织...</option>';

        if (isSuperAdmin) {
            try {
                const resp = await fetch('/api/admin/orgs?page=1&page_size=100');
                if (resp.ok) {
                    const data = await resp.json();
                    (data.organizations || []).forEach(org => {
                        const option = document.createElement('option');
                        option.value = org.id;
                        option.textContent = org.name;
                        select.appendChild(option);
                    });
                }
            } catch (e) {
                console.error('加载组织列表失败:', e);
            }
        } else if (managedOrgs.length > 0) {
            managedOrgs.forEach(org => {
                const option = document.createElement('option');
                option.value = org.id;
                option.textContent = org.name;
                select.appendChild(option);
            });
        }
    }

    async function loadUsers(keyword) {
        const select = document.getElementById('user-select');
        select.innerHTML = '<option value="">选择用户...</option>';
        state.user.selectedUserId = null;
        document.getElementById('user-table').style.display = 'none';
        document.getElementById('user-pagination').style.display = 'none';
        document.getElementById('user-empty').style.display = 'block';

        try {
            let url = '/api/admin/users?page=1&page_size=50';
            if (keyword) url += `&keyword=${encodeURIComponent(keyword)}`;

            const resp = await fetch(url);
            if (resp.ok) {
                const data = await resp.json();
                (data.users || []).forEach(user => {
                    const option = document.createElement('option');
                    option.value = user.id;
                    option.textContent = user.email;
                    select.appendChild(option);
                });
            }
        } catch (e) {
            console.error('加载用户列表失败:', e);
        }
    }

    function formatDateTime(isoString) {
        if (!isoString) {
            return '-';
        }
        const date = new Date(isoString);
        if (Number.isNaN(date.getTime())) {
            return '-';
        }
        const y = date.getFullYear();
        const m = String(date.getMonth() + 1).padStart(2, '0');
        const d = String(date.getDate()).padStart(2, '0');
        const h = String(date.getHours()).padStart(2, '0');
        const min = String(date.getMinutes()).padStart(2, '0');
        return `${y}-${m}-${d} ${h}:${min}`;
    }

    function debounce(fn, delay) {
        let timer = null;
        return function(...args) {
            clearTimeout(timer);
            timer = setTimeout(() => fn.apply(this, args), delay);
        };
    }

    function showError(msg) {
        document.getElementById('loading').style.display = 'none';
        document.getElementById('error-message').textContent = msg;
    }

    function clearError() {
        document.getElementById('error-message').textContent = '';
    }

    init();
})();
