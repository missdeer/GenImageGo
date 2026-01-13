(function() {
    let currentPage = 1;
    let totalPages = 1;
    let currentKeyword = '';
    let userPermissions = null;
    let isSuperAdmin = false;

    let pointsModalOrg = null;
    let nameModalOrg = null;
    let confirmCallback = null;

    async function init() {
        try {
            const resp = await fetch('/api/auth/me');
            if (!resp.ok) {
                window.location.href = '/login';
                return;
            }
            const user = await resp.json();
            userPermissions = user;
            isSuperAdmin = user.is_super_admin;

            if (!user.can_manage_users) {
                document.getElementById('loading').style.display = 'none';
                document.getElementById('no-permission').style.display = 'block';
                return;
            }

            if (isSuperAdmin) {
                document.getElementById('create-org-wrapper').style.display = 'block';
            }

            loadOrgs();
        } catch (e) {
            showError('加载失败: ' + e.message);
        }
    }

    async function loadOrgs() {
        document.getElementById('loading').style.display = 'block';
        document.getElementById('orgs-table').style.display = 'none';
        document.getElementById('error-message').textContent = '';

        let url = `/api/admin/orgs?page=${currentPage}&page_size=20`;
        if (currentKeyword) url += `&keyword=${encodeURIComponent(currentKeyword)}`;

        try {
            const resp = await fetch(url);
            if (!resp.ok) {
                const data = await resp.json();
                throw new Error(data.error || '请求失败');
            }
            const data = await resp.json();
            renderOrgs(data);
        } catch (e) {
            showError(e.message);
        } finally {
            document.getElementById('loading').style.display = 'none';
        }
    }

    function renderOrgs(data) {
        const tbody = document.getElementById('orgs-tbody');
        tbody.innerHTML = '';

        if (!data.organizations || data.organizations.length === 0) {
            tbody.innerHTML = '<tr><td colspan="6" class="empty-message">暂无组织数据</td></tr>';
            document.getElementById('orgs-table').style.display = 'table';
            document.getElementById('pagination').style.display = 'none';
            return;
        }

        data.organizations.forEach(org => {
            const tr = document.createElement('tr');
            const createdAt = new Date(org.created_at).toLocaleDateString('zh-CN');

            const values = [org.id, org.name, org.points, org.member_count, createdAt];
            values.forEach(value => {
                const td = document.createElement('td');
                td.textContent = value;
                tr.appendChild(td);
            });

            const actionsTd = document.createElement('td');
            if (isSuperAdmin) {
                const nameBtn = document.createElement('button');
                nameBtn.className = 'action-btn';
                nameBtn.textContent = '改名';
                nameBtn.addEventListener('click', () => openNameModal(org.id, org.name));

                const pointsBtn = document.createElement('button');
                pointsBtn.className = 'action-btn points';
                pointsBtn.textContent = '积分';
                pointsBtn.addEventListener('click', () => openPointsModal(org.id, org.name, org.points));

                const deleteBtn = document.createElement('button');
                deleteBtn.className = 'action-btn delete';
                deleteBtn.textContent = '解散';
                deleteBtn.addEventListener('click', () => deleteOrg(org.id, org.name));

                actionsTd.appendChild(nameBtn);
                actionsTd.appendChild(pointsBtn);
                actionsTd.appendChild(deleteBtn);
            }
            tr.appendChild(actionsTd);
            tbody.appendChild(tr);
        });

        document.getElementById('orgs-table').style.display = 'table';

        totalPages = data.total_pages || 1;
        currentPage = data.page;
        document.getElementById('page-info').textContent = `第 ${currentPage} 页 / 共 ${totalPages} 页`;
        document.getElementById('prev-btn').disabled = currentPage <= 1;
        document.getElementById('next-btn').disabled = currentPage >= totalPages;
        document.getElementById('pagination').style.display = 'flex';
    }

    function showError(msg) {
        document.getElementById('loading').style.display = 'none';
        document.getElementById('error-message').textContent = msg;
    }

    // Search
    document.getElementById('search-btn').addEventListener('click', () => {
        currentKeyword = document.getElementById('search-keyword').value.trim();
        currentPage = 1;
        loadOrgs();
    });

    document.getElementById('search-keyword').addEventListener('keypress', (e) => {
        if (e.key === 'Enter') {
            currentKeyword = document.getElementById('search-keyword').value.trim();
            currentPage = 1;
            loadOrgs();
        }
    });

    // Pagination
    document.getElementById('prev-btn').addEventListener('click', () => {
        if (currentPage > 1) {
            currentPage--;
            loadOrgs();
        }
    });

    document.getElementById('next-btn').addEventListener('click', () => {
        if (currentPage < totalPages) {
            currentPage++;
            loadOrgs();
        }
    });

    // Create org button
    document.getElementById('create-org-btn').addEventListener('click', () => {
        document.getElementById('create-name').value = '';
        document.getElementById('create-points').value = '0';
        document.getElementById('create-modal').style.display = 'flex';
    });

    window.closeCreateModal = function() {
        document.getElementById('create-modal').style.display = 'none';
    };

    window.submitCreateOrg = async function() {
        const name = document.getElementById('create-name').value.trim();
        const points = parseInt(document.getElementById('create-points').value) || 0;

        if (!name) {
            alert('请输入组织名称');
            return;
        }

        try {
            const resp = await fetch('/api/admin/orgs/create', {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({ name, points })
            });
            const data = await resp.json();
            if (!resp.ok) throw new Error(data.error || '创建失败');
            closeCreateModal();
            loadOrgs();
        } catch (e) {
            alert('创建失败: ' + e.message);
        }
    };

    // Points modal
    function openPointsModal(orgId, orgName, currentPoints) {
        pointsModalOrg = { id: orgId, name: orgName };
        document.getElementById('points-modal-name').textContent = orgName;
        document.getElementById('points-input').value = currentPoints;
        document.getElementById('points-modal').style.display = 'flex';
    }

    window.closePointsModal = function() {
        document.getElementById('points-modal').style.display = 'none';
        pointsModalOrg = null;
    };

    // Name modal
    function openNameModal(orgId, orgName) {
        nameModalOrg = { id: orgId, name: orgName };
        document.getElementById('name-modal-current').textContent = orgName;
        document.getElementById('name-input').value = orgName;
        document.getElementById('name-modal').style.display = 'flex';
    }

    window.closeNameModal = function() {
        document.getElementById('name-modal').style.display = 'none';
        nameModalOrg = null;
    };

    window.submitUpdateName = async function() {
        if (!nameModalOrg) return;
        const name = document.getElementById('name-input').value.trim();
        if (!name) {
            alert('请输入组织名称');
            return;
        }

        try {
            const resp = await fetch('/api/admin/orgs/update-name', {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({ organization_id: nameModalOrg.id, name })
            });
            const data = await resp.json();
            if (!resp.ok) throw new Error(data.error || '更新失败');
            closeNameModal();
            loadOrgs();
        } catch (e) {
            alert('更新失败: ' + e.message);
        }
    };

    window.submitUpdatePoints = async function() {
        if (!pointsModalOrg) return;
        const points = parseInt(document.getElementById('points-input').value);
        if (isNaN(points) || points < 0) {
            alert('请输入有效的积分值');
            return;
        }

        try {
            const resp = await fetch('/api/admin/orgs/update-points', {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({ organization_id: pointsModalOrg.id, points })
            });
            const data = await resp.json();
            if (!resp.ok) throw new Error(data.error || '更新失败');
            closePointsModal();
            loadOrgs();
        } catch (e) {
            alert('更新失败: ' + e.message);
        }
    };

    // Confirm modal
    function showConfirmModal(title, message, callback, danger) {
        document.getElementById('confirm-modal-title').textContent = title;
        document.getElementById('confirm-modal-message').textContent = message;
        const btn = document.getElementById('confirm-modal-btn');
        btn.className = danger ? 'btn-primary danger' : 'btn-primary';
        confirmCallback = callback;
        document.getElementById('confirm-modal').style.display = 'flex';
    }

    window.closeConfirmModal = function() {
        document.getElementById('confirm-modal').style.display = 'none';
        confirmCallback = null;
    };

    window.confirmAction = function() {
        if (confirmCallback) {
            confirmCallback();
        }
        closeConfirmModal();
    };

    // Delete org
    async function deleteOrg(orgId, orgName) {
        showConfirmModal(
            '解散组织',
            `确定要解散组织 "${orgName}" 吗？此操作将移除所有成员关系且不可恢复！`,
            async () => {
                try {
                    const resp = await fetch('/api/admin/orgs/delete', {
                        method: 'POST',
                        headers: { 'Content-Type': 'application/json' },
                        body: JSON.stringify({ organization_id: orgId })
                    });
                    const data = await resp.json();
                    if (!resp.ok) throw new Error(data.error || '解散失败');
                    loadOrgs();
                } catch (e) {
                    alert('解散失败: ' + e.message);
                }
            },
            true
        );
    }

    init();
})();
