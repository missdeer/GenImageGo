(function() {
    let currentPage = 1;
    let totalPages = 1;
    let currentKeyword = '';
    let currentOrgId = '';
    let userPermissions = null;
    let currentUserId = null;
    let allOrganizations = [];

    // Modal state
    let pointsModalUser = null;
    let confirmCallback = null;
    let orgsModalUser = null;
    let orgsModalMemberships = [];

    async function init() {
        try {
            const resp = await fetch('/api/auth/me');
            if (!resp.ok) {
                window.location.href = '/login';
                return;
            }
            const user = await resp.json();
            userPermissions = user;
            currentUserId = user.id;

            if (!user.can_manage_users) {
                document.getElementById('loading').style.display = 'none';
                document.getElementById('no-permission').style.display = 'block';
                return;
            }

            await loadOrganizations();
            loadUsers();
        } catch (e) {
            showError('加载失败: ' + e.message);
        }
    }

    async function loadOrganizations() {
        try {
            const resp = await fetch('/api/admin/organizations');
            if (!resp.ok) return;
            const data = await resp.json();
            if (data.organizations && data.organizations.length > 0) {
                allOrganizations = data.organizations;
                const orgFilter = document.getElementById('org-filter');
                data.organizations.forEach(org => {
                    const option = document.createElement('option');
                    option.value = org.id;
                    option.textContent = org.name;
                    orgFilter.appendChild(option);
                });
            }
        } catch (e) {
            console.error('加载组织列表失败:', e);
        }
    }

    async function loadUsers() {
        document.getElementById('loading').style.display = 'block';
        document.getElementById('users-table').style.display = 'none';
        document.getElementById('error-message').textContent = '';

        let url = `/api/admin/users?page=${currentPage}&page_size=20`;
        if (currentKeyword) url += `&keyword=${encodeURIComponent(currentKeyword)}`;
        if (currentOrgId) url += `&organization_id=${currentOrgId}`;

        try {
            const resp = await fetch(url);
            if (!resp.ok) {
                const data = await resp.json();
                throw new Error(data.error || '请求失败');
            }
            const data = await resp.json();
            renderUsers(data);
        } catch (e) {
            showError(e.message);
        } finally {
            document.getElementById('loading').style.display = 'none';
        }
    }

    function renderUsers(data) {
        const tbody = document.getElementById('users-tbody');
        tbody.innerHTML = '';

        if (!data.users || data.users.length === 0) {
            tbody.innerHTML = '<tr><td colspan="7" class="empty-message">暂无用户</td></tr>';
        } else {
            data.users.forEach(user => {
                const tr = document.createElement('tr');
                const isSelf = user.id === currentUserId;
                const isSuperAdmin = user.type === 1;
                const canOperate = !isSelf && (!isSuperAdmin || userPermissions.is_super_admin);

                let statusBadge = '';
                if (isSuperAdmin) {
                    statusBadge = '<span class="status-badge admin">超级管理员</span>';
                } else if (user.disabled) {
                    statusBadge = '<span class="status-badge disabled">已禁用</span>';
                } else {
                    statusBadge = '<span class="status-badge normal">正常</span>';
                }

                let actions = '';
                if (canOperate) {
                    if (user.disabled) {
                        actions += `<button class="action-btn enable" onclick="window.adminActions.toggleDisabled(${user.id}, '${escapeHtml(user.email)}', false)">启用</button>`;
                    } else {
                        actions += `<button class="action-btn disable" onclick="window.adminActions.toggleDisabled(${user.id}, '${escapeHtml(user.email)}', true)">禁用</button>`;
                    }
                    actions += `<button class="action-btn points" onclick="window.adminActions.openPointsModal(${user.id}, '${escapeHtml(user.email)}', ${user.points})">积分</button>`;
                    const orgsData = encodeURIComponent(JSON.stringify(user.organizations || []));
                    actions += `<button class="action-btn orgs" onclick="window.adminActions.openOrgsModal(${user.id}, '${escapeHtml(user.email)}', '${orgsData}')">组织</button>`;
                    if (!isSuperAdmin) {
                        actions += `<button class="action-btn delete" onclick="window.adminActions.deleteUser(${user.id}, '${escapeHtml(user.email)}')">删除</button>`;
                    }
                } else if (isSelf) {
                    actions = '<span style="color:var(--text-secondary);font-size:12px;">当前用户</span>';
                }

                tr.innerHTML = `
                    <td>${user.id}</td>
                    <td>${escapeHtml(user.email)}</td>
                    <td>${statusBadge}</td>
                    <td>${user.points}</td>
                    <td>${formatDate(user.created_at)}</td>
                    <td>${renderOrganizations(user.organizations)}</td>
                    <td>${actions}</td>
                `;
                tbody.appendChild(tr);
            });
        }

        document.getElementById('users-table').style.display = 'table';

        totalPages = data.total_pages || 1;
        currentPage = data.page;
        document.getElementById('page-info').textContent = `第 ${currentPage} 页 / 共 ${totalPages} 页`;
        document.getElementById('prev-btn').disabled = currentPage <= 1;
        document.getElementById('next-btn').disabled = currentPage >= totalPages;
        document.getElementById('pagination').style.display = 'flex';
    }

    function renderOrganizations(orgs) {
        if (!orgs || orgs.length === 0) return '-';
        return orgs.map(o => `${escapeHtml(o.name)}(${o.role === 1 ? '管理员' : '成员'})`).join(', ');
    }

    function escapeHtml(text) {
        if (!text) return '';
        const div = document.createElement('div');
        div.textContent = text;
        return div.innerHTML;
    }

    function formatDate(dateStr) {
        if (!dateStr) return '-';
        const d = new Date(dateStr);
        return d.toLocaleDateString('zh-CN');
    }

    function showError(msg) {
        document.getElementById('error-message').textContent = msg;
    }

    // Toggle user disabled status
    async function toggleDisabled(userId, email, disabled) {
        const action = disabled ? '禁用' : '启用';
        showConfirmModal(
            `${action}用户`,
            `确定要${action}用户 "${email}" 吗？${disabled ? '禁用后该用户将无法登录。' : ''}`,
            async () => {
                try {
                    const resp = await fetch('/api/admin/users/toggle-disabled', {
                        method: 'POST',
                        headers: { 
                            'Content-Type': 'application/json',
                            'X-CSRF-Token': getCSRFToken()
                        },
                        body: JSON.stringify({ user_id: userId, disabled: disabled })
                    });
                    const data = await resp.json();
                    if (!resp.ok) throw new Error(data.error || '操作失败');
                    loadUsers();
                } catch (e) {
                    alert('操作失败: ' + e.message);
                }
            }
        );
    }

    // Delete user
    async function deleteUser(userId, email) {
        showConfirmModal(
            '删除用户',
            `确定要删除用户 "${email}" 吗？此操作不可恢复！`,
            async () => {
                try {
                    const resp = await fetch('/api/admin/users/delete', {
                        method: 'POST',
                        headers: { 
                            'Content-Type': 'application/json',
                            'X-CSRF-Token': getCSRFToken()
                        },
                        body: JSON.stringify({ user_id: userId })
                    });
                    const data = await resp.json();
                    if (!resp.ok) throw new Error(data.error || '删除失败');
                    loadUsers();
                } catch (e) {
                    alert('删除失败: ' + e.message);
                }
            },
            true
        );
    }

    // Points modal
    function openPointsModal(userId, email, currentPoints) {
        pointsModalUser = { id: userId, email: email };
        document.getElementById('points-modal-email').textContent = email;
        document.getElementById('points-input').value = currentPoints;
        document.getElementById('points-modal').style.display = 'flex';
    }

    function closePointsModal() {
        document.getElementById('points-modal').style.display = 'none';
        pointsModalUser = null;
    }

    async function submitUpdatePoints() {
        if (!pointsModalUser) return;
        const points = parseInt(document.getElementById('points-input').value, 10);
        if (isNaN(points) || points < 0) {
            alert('请输入有效的积分值');
            return;
        }
        try {
            const resp = await fetch('/api/admin/users/update-points', {
                method: 'POST',
                headers: { 
                    'Content-Type': 'application/json',
                    'X-CSRF-Token': getCSRFToken()
                },
                body: JSON.stringify({ user_id: pointsModalUser.id, points: points })
            });
            const data = await resp.json();
            if (!resp.ok) throw new Error(data.error || '操作失败');
            closePointsModal();
            loadUsers();
        } catch (e) {
            alert('修改失败: ' + e.message);
        }
    }

    // Confirm modal
    function showConfirmModal(title, message, callback, isDanger) {
        document.getElementById('confirm-modal-title').textContent = title;
        document.getElementById('confirm-modal-message').textContent = message;
        const btn = document.getElementById('confirm-modal-btn');
        btn.className = 'btn-primary' + (isDanger ? ' danger' : '');
        btn.style.background = '';
        confirmCallback = callback;
        document.getElementById('confirm-modal').style.display = 'flex';
    }

    function closeConfirmModal() {
        document.getElementById('confirm-modal').style.display = 'none';
        confirmCallback = null;
    }

    function confirmAction() {
        if (confirmCallback) {
            confirmCallback();
        }
        closeConfirmModal();
    }

    // Organizations modal
    async function openOrgsModal(userId, email, encodedOrgs) {
        orgsModalUser = { id: userId, email: email };
        const currentOrgs = JSON.parse(decodeURIComponent(encodedOrgs));
        orgsModalMemberships = (currentOrgs || []).map(o => ({
            organization_id: Number(o.id),
            role: Number(o.role),
            name: o.name
        }));
        if (allOrganizations.length === 0) {
            await loadOrganizations();
        }
        document.getElementById('orgs-modal-email').textContent = email;
        renderOrgsModalList();
        updateAddOrgSelect();
        document.getElementById('orgs-modal').style.display = 'flex';
    }

    function closeOrgsModal() {
        document.getElementById('orgs-modal').style.display = 'none';
        orgsModalUser = null;
        orgsModalMemberships = [];
    }

    function renderOrgsModalList() {
        const container = document.getElementById('orgs-list');
        if (orgsModalMemberships.length === 0) {
            container.innerHTML = '<p style="color:var(--text-secondary);">无所属组织</p>';
        } else {
            container.innerHTML = orgsModalMemberships.map((m, idx) => {
                const org = allOrganizations.find(o => Number(o.id) === Number(m.organization_id));
                const orgName = org ? org.name : m.name || `组织#${m.organization_id}`;
                return `
                    <div class="org-item" style="display:flex;align-items:center;gap:12px;padding:8px 0;border-bottom:1px solid var(--border-color);">
                        <span style="flex:1;">${escapeHtml(orgName)}</span>
                        <select onchange="window.updateOrgRole(${idx}, this.value)" style="padding:4px 8px;border:1px solid var(--border-color);border-radius:4px;background:var(--bg-primary);color:var(--text-primary);">
                            <option value="0" ${Number(m.role) === 0 ? 'selected' : ''}>成员</option>
                            <option value="1" ${Number(m.role) === 1 ? 'selected' : ''}>管理员</option>
                        </select>
                        <button class="action-btn delete" onclick="window.removeOrgFromList(${idx})">移除</button>
                    </div>
                `;
            }).join('');
        }
    }

    function updateAddOrgSelect() {
        const select = document.getElementById('add-org-select');
        const currentOrgIds = orgsModalMemberships.map(m => Number(m.organization_id));
        const availableOrgs = allOrganizations.filter(o => !currentOrgIds.includes(Number(o.id)));

        select.innerHTML = '<option value="">选择组织...</option>';
        availableOrgs.forEach(org => {
            const option = document.createElement('option');
            option.value = org.id;
            option.textContent = org.name;
            select.appendChild(option);
        });

        document.getElementById('add-org-section').style.display = availableOrgs.length > 0 ? 'block' : 'none';
    }

    function addOrgToList() {
        const select = document.getElementById('add-org-select');
        const selectedOption = select.options[select.selectedIndex];
        if (!selectedOption || selectedOption.value === '') {
            return;
        }
        const orgId = Number(selectedOption.value);
        if (!Number.isFinite(orgId)) {
            return;
        }

        if (orgsModalMemberships.some(m => Number(m.organization_id) === orgId)) {
            return;
        }
        const org = allOrganizations.find(o => Number(o.id) === orgId);
        const orgName = org ? org.name : (selectedOption.textContent || `组织#${orgId}`);

        orgsModalMemberships.push({ organization_id: orgId, role: 0, name: orgName });
        renderOrgsModalList();
        updateAddOrgSelect();
    }

    function removeOrgFromList(idx) {
        orgsModalMemberships.splice(idx, 1);
        renderOrgsModalList();
        updateAddOrgSelect();
    }

    function updateOrgRole(idx, role) {
        orgsModalMemberships[idx].role = parseInt(role, 10);
    }

    async function submitUpdateOrgs() {
        if (!orgsModalUser) {
            return;
        }
        const payload = {
            user_id: orgsModalUser.id,
            memberships: orgsModalMemberships.map(m => ({
                organization_id: m.organization_id,
                role: m.role
            }))
        };
        try {
            const resp = await fetch('/api/admin/users/update-memberships', {
                method: 'POST',
                headers: { 
                    'Content-Type': 'application/json',
                    'X-CSRF-Token': getCSRFToken()
                },
                body: JSON.stringify(payload)
            });
            const data = await resp.json();
            if (!resp.ok) throw new Error(data.error || '操作失败');
            closeOrgsModal();
            loadUsers();
        } catch (e) {
            alert('修改失败: ' + e.message);
        }
    }

    // Event listeners
    document.getElementById('search-btn').addEventListener('click', () => {
        currentKeyword = document.getElementById('search-keyword').value.trim();
        currentPage = 1;
        loadUsers();
    });

    document.getElementById('search-keyword').addEventListener('keypress', (e) => {
        if (e.key === 'Enter') {
            document.getElementById('search-btn').click();
        }
    });

    document.getElementById('org-filter').addEventListener('change', (e) => {
        currentOrgId = e.target.value;
        currentPage = 1;
        loadUsers();
    });

    document.getElementById('prev-btn').addEventListener('click', () => {
        if (currentPage > 1) {
            currentPage--;
            loadUsers();
        }
    });

    document.getElementById('next-btn').addEventListener('click', () => {
        if (currentPage < totalPages) {
            currentPage++;
            loadUsers();
        }
    });

    // Expose functions to window for onclick handlers
    window.adminActions = {
        toggleDisabled,
        deleteUser,
        openPointsModal,
        openOrgsModal
    };
    window.closePointsModal = closePointsModal;
    window.submitUpdatePoints = submitUpdatePoints;
    window.closeConfirmModal = closeConfirmModal;
    window.confirmAction = confirmAction;
    window.closeOrgsModal = closeOrgsModal;
    window.submitUpdateOrgs = submitUpdateOrgs;
    window.addOrgToList = addOrgToList;
    window.removeOrgFromList = removeOrgFromList;
    window.updateOrgRole = updateOrgRole;

    init();
})();
