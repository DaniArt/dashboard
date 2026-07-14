const API_BASE = window.location.origin + '/qa-dashboard';
const POLL_INTERVAL = 30000;
let dashboardData = null, tasksData = null, mrData = null, confData = null;
let currentPage = 'dashboard';

function getMonthOptions() {
    const months = [];
    const now = new Date();
    const currentYM = `${now.getFullYear()}-${String(now.getMonth() + 1).padStart(2, '0')}`;
    let d = new Date(2026, 0, 1);
    while (d <= now) {
        const year = d.getFullYear();
        const month = String(d.getMonth() + 1).padStart(2, '0');
        const val = `${year}-${month}`;
        // Skip current month - already covered by "Текущий месяц" option
        if (val !== currentYM) {
            const label = d.toLocaleDateString('ru-RU', { year: 'numeric', month: 'long' });
            months.push({ val, label });
        }
        d.setMonth(d.getMonth() + 1);
    }
    return months.reverse();
}

document.addEventListener('DOMContentLoaded', () => {
    initMonthFilter('dashboardMonthFilter', () => fetchDashboard());
 const savedPage = location.hash.replace('#', '') || localStorage.getItem('qa-dashboard-page') || 'dashboard';
 navigateTo(savedPage);
 setInterval(function() { if (currentPage === 'dashboard') fetchDashboard(); }, POLL_INTERVAL);
});

function navigateTo(page) {
    currentPage = page;
 location.hash = page; localStorage.setItem("qa-dashboard-page", page);
    document.querySelectorAll('.page').forEach(p => p.classList.add('hidden'));
    document.getElementById(`page-${page}`).classList.remove('hidden');
    document.querySelectorAll('.nav-item').forEach(n => n.classList.remove('active'));
    document.querySelector(`[data-page="${page}"]`).classList.add('active');
    if (page === 'dashboard') fetchDashboard();
    if (page === 'tasks') loadTasks();
    if (page === 'merge-requests') loadMergeRequests();
    if (page === 'confluence') loadConfluence();
    if (page === 'attendance') loadAttendance();
    if (page === 'candidates') loadCandidates();
 
}

function initMonthFilter(id, onChange) {
    const el = document.getElementById(id);
    if (!el || el.dataset.init) return;
    el.dataset.init = '1';
    const months = getMonthOptions();
    el.innerHTML = `<option value="">Текущий месяц</option><option value="all">За весь период</option>` +
        months.map(m => `<option value="${m.val}">${m.label}</option>`).join('');
    el.addEventListener('change', onChange);
}

// === DASHBOARD ===
async function fetchDashboard() {
    try {
        showDashboardLoading(true);
        const month = document.getElementById('dashboardMonthFilter')?.value || '';
        const url = month ? `${API_BASE}/api/dashboard?month=${month}` : `${API_BASE}/api/dashboard`;
        const resp = await fetch(url, { cache: 'no-store' });
        if (!resp.ok) throw new Error(`HTTP ${resp.status}`);
        dashboardData = await resp.json();
        renderDashboard(dashboardData);
        updateConnectionStatus(true);
    } catch (err) { console.error('Fetch error:', err); updateConnectionStatus(false); }
    finally { showDashboardLoading(false); }
}

function showDashboardLoading(show) {
    let overlay = document.getElementById('dashboardLoadingOverlay');
    if (show) {
        if (!overlay) {
            overlay = document.createElement('div');
            overlay.id = 'dashboardLoadingOverlay';
            overlay.className = 'loading-overlay';
            overlay.innerHTML = '<div class="loading-overlay-content"><div class="spinner"></div><span>Загрузка данных...</span></div>';
            document.getElementById('page-dashboard').appendChild(overlay);
        }
        overlay.style.display = 'flex';
    } else {
        if (overlay) overlay.style.display = 'none';
    }
}

function renderDashboard(data) {
    // Summary - linked to filter
    document.getElementById('totalActive').textContent = data.summary.total_active_tasks;
    document.getElementById('totalCompleted').textContent = data.summary.total_completed_month;
    document.getElementById('totalMRs').textContent = data.summary.total_mrs_month;
    document.getElementById('totalAlerts').textContent = data.summary.total_alerts;

    renderGauges(data.employees, data.summary);
    renderAlerts(data.alerts);
    renderEmployees(data.employees);
    renderDashboardConclusion(data);

    const dt = new Date(data.last_updated);
    document.getElementById('lastUpdated').textContent = dt.toLocaleTimeString('ru-RU', { hour: '2-digit', minute: '2-digit' });
}

function renderGauges(employees, summary) {
    const container = document.getElementById('gaugesSection');
    if (!container) return;

    let totalActive = 0, totalCompleted = 0, totalStale = 0, totalMR = 0, totalMRMerged = 0;
    employees.forEach(e => {
        totalActive += e.tasks.active_tasks;
        totalCompleted += e.tasks.completed_month;
        totalStale += e.tasks.stale_tasks;
        totalMR += e.gitlab.mrs_created_month;
        totalMRMerged += e.gitlab.mrs_merged_month;
    });

    const total = totalActive + totalCompleted;
    const completionRate = total > 0 ? Math.round(totalCompleted / total * 100) : 0;
    const noStaleRate = totalActive > 0 ? Math.max(0, 100 - Math.round(totalStale / totalActive * 100)) : 100;
    const mrMergeRate = totalMR > 0 ? Math.round(totalMRMerged / totalMR * 100) : 0;

    container.innerHTML = `<div class="gauges-grid">
        ${createGauge('Исполнение', completionRate, 'green', `${totalCompleted} из ${total} задач завершено`)}
        ${createGauge('Без зависаний', noStaleRate, noStaleRate > 70 ? 'green' : noStaleRate > 40 ? 'orange' : 'red', `${totalStale} задач стоят >5 дней`)}
        ${createGauge('MR Merged', mrMergeRate, 'blue', `${totalMRMerged} из ${totalMR} MR влиты`)}
        ${createGauge('Загрузка', Math.min(100, Math.round(totalActive / Math.max(employees.length, 1) * 10)), 'purple', `~${Math.round(totalActive / Math.max(employees.length, 1))} задач на человека`)}
    </div>`;
}

function createGauge(label, value, color, detail) {
    const circumference = 2 * Math.PI * 54;
    const offset = circumference - (value / 100) * circumference;
    const colorVar = `var(--accent-${color})`;
    return `<div class="gauge-item">
        <svg class="gauge-svg" viewBox="0 0 120 120">
            <circle cx="60" cy="60" r="54" fill="none" stroke="var(--bg-primary)" stroke-width="10"/>
            <circle cx="60" cy="60" r="54" fill="none" stroke="${colorVar}" stroke-width="10" stroke-linecap="round"
                stroke-dasharray="${circumference}" stroke-dashoffset="${offset}"
                transform="rotate(-90 60 60)" style="transition:stroke-dashoffset 1s ease"/>
            <text x="60" y="56" text-anchor="middle" fill="var(--text-primary)" font-size="22" font-weight="700">${value}%</text>
            <text x="60" y="74" text-anchor="middle" fill="var(--text-secondary)" font-size="10">${label}</text>
        </svg>
        <div class="gauge-detail">${detail}</div>
    </div>`;
}

function renderDashboardConclusion(data) {
    const el = document.getElementById('dashboardConclusion');
    if (!el) return;
    const issues = [];
    let totalStale = 0, totalNoActivity = 0;
    data.employees.forEach(e => {
        totalStale += e.tasks.stale_tasks;
        if (e.gitlab.mrs_created_month === 0 && e.employee.gitlab_groups && e.employee.gitlab_groups.length > 0) totalNoActivity++;
    });
    if (totalStale > 0) issues.push(`${totalStale} задач зависли (>5 дней в одном статусе) — провести разбор`);
    if (totalNoActivity > 0) issues.push(`${totalNoActivity} сотрудников без активности в GitLab`);
    if (data.summary.critical_alerts > 0) issues.push(`${data.summary.critical_alerts} критических алертов`);

    el.style.display = 'block';
    if (issues.length === 0) {
        el.className = 'conclusion-banner';
        el.innerHTML = `<div class="conclusion-title"><i class="fas fa-check-circle"></i> Общее заключение</div><div class="conclusion-items"><div class="conclusion-item"><span class="issue-text" style="color:var(--accent-green)">Команда работает в нормальном режиме. Замечаний нет.</span></div></div>`;
    } else {
        el.className = 'conclusion-banner has-issues collapsed';
        el.innerHTML = `<div class="conclusion-title" onclick="this.parentElement.classList.toggle('collapsed')"><i class="fas fa-exclamation-circle"></i> Общее заключение и рекомендации <span class="conclusion-count">${issues.length}</span><i class="fas fa-chevron-down conclusion-chevron"></i></div><div class="conclusion-items">${issues.map(i => `<div class="conclusion-item"><span class="issue-text">${i}</span></div>`).join('')}</div>`;
    }
}

function renderAlerts(alerts) {
    const section = document.getElementById('alertsSection');
    const list = document.getElementById('alertsList');
    if (!alerts || alerts.length === 0) { section.style.display = 'none'; return; }
    section.style.display = 'block';
    list.innerHTML = alerts.map(a => `<div class="alert-item ${a.severity}"><span class="alert-badge ${a.severity}">${a.severity==='critical'?'КРИТ':'ВНИМ'}</span><span class="alert-employee">${a.employee}</span><span class="alert-message">${a.message}</span>${a.task_url?`<a href="${a.task_url}" target="_blank" class="alert-link"><i class="fas fa-external-link-alt"></i></a>`:''}</div>`).join('');
}

function renderEmployees(employees) {
    const grid = document.getElementById('employeesGrid');
    if (!employees || employees.length === 0) { grid.innerHTML = '<div class="loading-state"><p>Нет данных</p></div>'; return; }

    // Save expanded state of ART groups before re-render
    const expandedGroups = new Set();
    grid.querySelectorAll('.art-group-grid').forEach(el => {
        if (el.style.display !== 'none') expandedGroups.add(el.id);
    });

    // Group by ART
    const artGroups = {};
    employees.forEach(emp => {
        const art = emp.employee.art || 'Без поезда';
        if (!artGroups[art]) artGroups[art] = [];
        artGroups[art].push(emp);
    });

    // Define ART order and colors
    const artOrder = ['ART Digital Lending', 'ART Digital Daily Banking', 'ART SME Lending', 'ART Business Banking Transact', 'ART Platform', 'Без поезда'];
    const artColors = {
        'ART Digital Lending': 'var(--accent-purple)',
        'ART Digital Daily Banking': 'var(--accent-blue)',
        'ART SME Lending': 'var(--accent-orange)',
        'ART Business Banking Transact': 'var(--accent-teal)',
        'ART Platform': 'var(--accent-magenta)',
        'Без поезда': 'var(--text-muted)',
    };

    let html = '';
    artOrder.forEach(art => {
        const emps = artGroups[art];
        if (!emps || emps.length === 0) return;
        const color = artColors[art] || 'var(--text-muted)';
        const groupId = 'art-' + art.replace(/[^a-zA-Z]/g, '');
        const isExpanded = expandedGroups.has(groupId);
        html += `<div class="art-group">
            <div class="art-group-header" onclick="toggleArtGroup('${groupId}')">
                <span class="art-group-dot" style="background:${color}"></span>
                <span class="art-group-title">${art}</span>
                <span class="art-group-count">${emps.length} чел.</span>
                <i class="fas ${isExpanded ? 'fa-chevron-down' : 'fa-chevron-right'} art-group-chevron" id="${groupId}-chevron"></i>
            </div>
            <div class="art-group-grid" id="${groupId}" style="display:${isExpanded ? '' : 'none'};">`;
        emps.forEach(emp => {
            const initials = getInitials(emp.employee.name);
            const hasAlerts = emp.alerts && emp.alerts.length > 0;
            const totalTasks = emp.tasks.total_tasks;
            const loadLevel = totalTasks > 15 ? 'Высокая' : totalTasks > 8 ? 'Средняя' : 'Низкая';
            const loadColor = totalTasks > 15 ? 'var(--accent-red)' : totalTasks > 8 ? 'var(--accent-orange)' : 'var(--accent-green)';
            const teamLabel = emp.employee.team ? `<div class="employee-team">${emp.employee.team}</div>` : '';
            html += `<div class="employee-card ${hasAlerts?'has-alerts':''}">
                <div class="employee-header">
                    <div class="employee-info"><div class="employee-avatar">${initials}</div><div><div class="employee-name">${emp.employee.name}</div><div class="employee-role">${emp.employee.role}</div>${teamLabel}</div></div>
                    ${hasAlerts?`<span class="employee-alerts-badge"><i class="fas fa-exclamation-triangle"></i></span>`:''}
                </div>
                <div class="metrics-grid">
                    <div class="metric-item"><div class="metric-value">${emp.tasks.backlog_tasks}</div><div class="metric-label">Бэклог</div></div>
                    <div class="metric-item"><div class="metric-value">${emp.tasks.active_tasks}</div><div class="metric-label">Активные</div></div>
                    <div class="metric-item"><div class="metric-value">${emp.tasks.closed_tasks}</div><div class="metric-label">Закрытые</div></div>
                    <div class="metric-item"><div class="metric-value">${emp.tasks.total_tasks}</div><div class="metric-label">Всего задач</div></div>
                    <div class="metric-item"><div class="metric-value" style="color:${emp.tasks.stale_tasks>0?'var(--accent-red)':'var(--accent-green)'}">${emp.tasks.stale_tasks}</div><div class="metric-label">Зависшие</div></div>
                    <div class="metric-item"><div class="metric-value" style="color:${loadColor};font-size:14px">${loadLevel}</div><div class="metric-label">Загрузка</div></div>
                </div>
            </div>`;
        });
        html += `</div></div>`;
    });
    grid.innerHTML = html;
}

function toggleArtGroup(groupId) {
    const el = document.getElementById(groupId);
    const chevron = document.getElementById(groupId + '-chevron');
    if (el.style.display === 'none') {
        el.style.display = '';
        chevron.classList.remove('fa-chevron-right');
        chevron.classList.add('fa-chevron-down');
    } else {
        el.style.display = 'none';
        chevron.classList.remove('fa-chevron-down');
        chevron.classList.add('fa-chevron-right');
    }
}

// === TASKS ===
async function loadTasks() {
    initMonthFilter('tasksMonthFilter', () => loadTasks());
    const month = document.getElementById('tasksMonthFilter')?.value || '';
    const url = month ? `${API_BASE}/api/tasks?month=${month}` : `${API_BASE}/api/tasks`;
    try { const r = await fetch(url); tasksData = await r.json(); renderTasksPage(tasksData); } catch(e) { console.error(e); }
}
function renderTasksPage(data) {
    const ef = document.getElementById('tasksFilterEmployee'), sf = document.getElementById('tasksFilterStatus'), tf = document.getElementById('tasksFilterType');

    // Save current filter values before rebuilding
    const prevEmp = ef.value;
    const prevStatus = sf.value;
    const prevType = tf.value;

    const emps = new Set(), stats = new Set(), types = new Set();
    let all = [];
    data.forEach(d => { emps.add(d.employee); (d.issues||[]).forEach(i => { stats.add(i.status); types.add(i.type); all.push({...i, employee: d.employee}); }); });
    ef.innerHTML = '<option value="all">Все сотрудники</option>'+[...emps].sort().map(e=>`<option value="${e}">${e}</option>`).join('');
    sf.innerHTML = '<option value="all">Все статусы</option>'+[...stats].map(s=>`<option value="${s}">${s}</option>`).join('');
    tf.innerHTML = '<option value="all">Все типы</option>'+[...types].map(t=>`<option value="${t}">${t}</option>`).join('');

    // Restore previous filter values if they still exist in options
    if (prevEmp && [...ef.options].some(o => o.value === prevEmp)) ef.value = prevEmp;
    if (prevStatus && [...sf.options].some(o => o.value === prevStatus)) sf.value = prevStatus;
    if (prevType && [...tf.options].some(o => o.value === prevType)) tf.value = prevType;

    renderTasksConclusion(data);

    // Apply filters to the data
    if (ef.value !== 'all') all = all.filter(i => i.employee === ef.value);
    if (sf.value !== 'all') all = all.filter(i => i.status === sf.value);
    if (tf.value !== 'all') all = all.filter(i => i.type === tf.value);

    renderTasksTable(all);
}
function filterTasks() {
    if (!tasksData) return;
    let all = []; tasksData.forEach(d=>{(d.issues||[]).forEach(i=>{all.push({...i,employee:d.employee});});});
    const e=document.getElementById('tasksFilterEmployee').value, s=document.getElementById('tasksFilterStatus').value, t=document.getElementById('tasksFilterType').value;
    const search = (document.getElementById('tasksSearch')?.value || '').toLowerCase();
    if(e!=='all')all=all.filter(i=>i.employee===e); if(s!=='all')all=all.filter(i=>i.status===s); if(t!=='all')all=all.filter(i=>i.type===t);
    if(search) all=all.filter(i=>(i.key+' '+i.summary+' '+i.employee+' '+i.status+' '+i.type).toLowerCase().includes(search));
    renderTasksTable(all);
}
function renderTasksConclusion(data) {
    const el = document.getElementById('tasksConclusion'); if(!el) return;
    const items = data.filter(d=>d.conclusion&&!d.conclusion.startsWith('Задачи обрабатываются'));
    if(items.length===0){el.style.display='block';el.className='conclusion-banner';el.innerHTML=`<div class="conclusion-title"><i class="fas fa-check-circle"></i> Заключение</div><div class="conclusion-items"><div class="conclusion-item"><span class="issue-text" style="color:var(--accent-green)">Задачи обрабатываются в нормальном режиме.</span></div></div>`;return;}
    el.style.display='block';el.className='conclusion-banner has-issues collapsed';
    el.innerHTML=`<div class="conclusion-title" onclick="this.parentElement.classList.toggle('collapsed')"><i class="fas fa-exclamation-circle"></i> Заключение и рекомендации <span class="conclusion-count">${items.length}</span><i class="fas fa-chevron-down conclusion-chevron"></i></div><div class="conclusion-items">${data.map(d=>d.conclusion&&!d.conclusion.startsWith('Задачи обрабатываются')?`<div class="conclusion-item"><span class="emp-name">${d.employee}</span><span class="issue-text">${d.conclusion.replace('Рекомендации: ','')}</span></div>`:'').filter(Boolean).join('')}</div>`;
}
function renderTasksTable(issues) {
    const tb = document.getElementById('tasksTableBody');
    if(!issues||!issues.length){tb.innerHTML='<tr><td colspan="10" style="text-align:center;padding:40px;color:var(--text-secondary)">Нет задач</td></tr>';return;}
    tb.innerHTML=issues.map(i=>{
        const d=getDays(i.updated||i.status_since);
        const statusLower = (i.status||'').toLowerCase();
        // Замечания только для активных статусов
        const activeStatuses = ['открытый','на анализе','в работе','анализ','analysis','analytics'];
        const isActive = activeStatuses.some(s => statusLower.includes(s));
        const dc = isActive ? (d>=10?'critical':d>=5?'warning':'ok') : 'ok';
        const c = [];
        if (isActive && d>=5) c.push(`Зависла ${d}д`);
        // Цвет статуса
        const sc = getStatusColorClass(statusLower);
        const createdDate = i.created ? new Date(i.created).toLocaleDateString('ru-RU') : '-';
        const updatedDate = i.updated ? new Date(i.updated).toLocaleDateString('ru-RU') : '-';
        return`<tr><td><a href="${i.url}" target="_blank" class="task-key">${i.key}</a></td><td style="font-size:12px">${i.employee}</td><td style="max-width:260px;overflow:hidden;text-overflow:ellipsis;white-space:nowrap">${i.summary}</td><td style="font-size:12px;color:var(--text-secondary)">${i.type}</td><td><span class="task-status-badge ${sc}">${i.status}</span></td><td style="font-size:12px">${createdDate}</td><td style="font-size:12px">${updatedDate}</td><td><span class="days-badge ${dc}">${isActive?d+'д':''}</span></td><td>${c.length?`<span class="task-comment"><i class="fas fa-exclamation-circle"></i> ${c.join('; ')}</span>`:''}</td><td><button class="btn-icon" onclick="loadTaskComments('${i.key}',this)" title="Комментарии"><i class="fas fa-comment"></i></button></td></tr>`;
    }).join('');
}

function getStatusColorClass(status) {
    // Готово/Done/Выполнено/Ready for development — зелёный
    if (status.includes('готово') || status.includes('done') || status.includes('выполнено') || status.includes('ready for development') || status.includes('resolved') || status.includes('закрыт') || status.includes('завершено')) return 'status-green';
    // Анализ/В работе/Открытый — синий
    if (status.includes('анализ') || status.includes('в работе') || status.includes('открыт') || status.includes('analysis') || status.includes('analytics') || status.includes('in progress')) return 'status-blue';
    // Отмена — красный
    if (status.includes('отмен') || status.includes('cancel') || status.includes('rejected')) return 'status-red';
    // Backlog/Сделать — серый
    if (status.includes('backlog') || status.includes('сделать') || status.includes('to do')) return 'status-gray';
    // Blocked — оранжевый
    if (status.includes('block')) return 'status-orange';
    // Остальное — фиолетовый
    return 'status-purple';
}

// === MERGE REQUESTS / GIT ACTIVITIES ===
function switchGitTab(tab) {
    document.getElementById('git-tab-mr').style.display = tab === 'mr' ? '' : 'none';
    document.getElementById('git-tab-pushes').style.display = tab === 'pushes' ? '' : 'none';
    document.getElementById('subtab-mr').classList.toggle('active', tab === 'mr');
    document.getElementById('subtab-pushes').classList.toggle('active', tab === 'pushes');
    if (tab === 'pushes') loadPushHistory();
}

let pushData = null;
async function loadPushHistory() {
    initMonthFilter('pushMonthFilter', () => loadPushHistory());
    document.getElementById('pushTableBody').innerHTML = '<tr><td colspan="5" class="loading-cell"><div class="spinner"></div></td></tr>';
    const month = document.getElementById('pushMonthFilter')?.value || '';
    const params = month === 'all' ? '?month=all' : month ? `?month=${month}` : '';
    try {
        const url = `${API_BASE}/api/pushes${params}`;
        const r = await fetch(url);
        const rows = await r.json();
        pushData = rows || [];
        renderPushTable(pushData);
        // Populate filter
        const ef = document.getElementById('pushFilterEmployee');
        const names = [...new Set(pushData.map(r => r.employee))];
        ef.innerHTML = '<option value="all">Все сотрудники</option>' + names.sort().map(n => `<option value="${n}">${n}</option>`).join('');
    } catch(e) { console.error(e); }
}

function filterPushes() {
    if (!pushData) return;
    let filtered = [...pushData];
    const emp = document.getElementById('pushFilterEmployee')?.value;
    const search = document.getElementById('pushSearch')?.value?.toLowerCase() || '';
    if (emp && emp !== 'all') filtered = filtered.filter(r => r.employee === emp);
    if (search) filtered = filtered.filter(r => (r.repo||'').toLowerCase().includes(search) || (r.commit_msg||'').toLowerCase().includes(search));
    renderPushTable(filtered);
}

let pushSortDir = 'desc';
function sortPushByDate() {
    pushSortDir = pushSortDir === 'desc' ? 'asc' : 'desc';
    document.getElementById('pushSortDate').textContent = pushSortDir === 'asc' ? '↑' : '↓';
    renderPushTable(pushData);
}

function renderPushTable(rows) {
    const tb = document.getElementById('pushTableBody');
    if (!rows || !rows.length) { tb.innerHTML = '<tr><td colspan="5" style="text-align:center;padding:40px;color:var(--text-secondary)">Нет данных</td></tr>'; return; }

    // Sort by date
    rows.sort((a, b) => {
        const da = a.date || '', db = b.date || '';
        return pushSortDir === 'desc' ? db.localeCompare(da) : da.localeCompare(db);
    });

    // Group by employee, sorted alphabetically
    const groups = {};
    rows.forEach(r => { if (!groups[r.employee]) groups[r.employee] = []; groups[r.employee].push(r); });
    const sortedNames = Object.keys(groups).sort();

    let html = '';
    sortedNames.forEach(name => {
        const items = groups[name];
        const groupId = 'push-' + name.replace(/[^a-zA-Zа-яА-ЯёЁ0-9]/g, '');
        html += `<tr class="group-header-row" onclick="toggleTableGroup('${groupId}')"><td colspan="5" class="group-header-cell"><i class="fas fa-chevron-right group-chevron" id="${groupId}-chev"></i><strong>${name}</strong><span class="group-count">${items.length} пушей</span></td></tr>`;
        items.forEach(r => {
            html += `<tr class="group-row ${groupId}" style="display:none;"><td>${r.employee}</td><td><a href="https://gitlab.fortebank.com/${r.repo}" target="_blank" class="task-key">${r.repo || '-'}</a></td><td style="max-width:300px;overflow:hidden;text-overflow:ellipsis;white-space:nowrap" title="${r.commit_msg||''}">${r.url ? '<a href="' + r.url + '" target="_blank" class="task-key">' + (r.commit_msg||'-') + '</a>' : (r.commit_msg||'-')}</td><td style="font-size:12px;white-space:nowrap">${r.date || '-'}</td><td style="font-size:12px;color:var(--text-secondary)">${r.team || ''}</td></tr>`;
        });
    });
    tb.innerHTML = html;
}

function toggleTableGroup(groupId) {
    const rows = document.querySelectorAll('.' + groupId);
    const chevron = document.getElementById(groupId + '-chev');
    const visible = rows[0] && rows[0].style.display !== 'none';
    rows.forEach(r => r.style.display = visible ? 'none' : '');
    if (chevron) {
        chevron.classList.toggle('fa-chevron-right', visible);
        chevron.classList.toggle('fa-chevron-down', !visible);
    }
}

async function loadMergeRequests() {
    initMonthFilter('mrMonthFilter', () => loadMergeRequests());
    document.getElementById('mrTableBody').innerHTML = '<tr><td colspan="9" class="loading-cell"><div class="spinner"></div></td></tr>';
    const month = document.getElementById('mrMonthFilter')?.value || '';
    const url = month ? `${API_BASE}/api/merge-requests?month=${month}` : `${API_BASE}/api/merge-requests`;
    try { const r = await fetch(url); mrData = await r.json(); renderMRPage(mrData); } catch(e) { console.error(e); }
}
function renderMRPage(data) {
    const empFilter = document.getElementById('mrFilterEmployee');
    const prevEmp = empFilter.value;

    empFilter.innerHTML='<option value="all">Все сотрудники</option>'+data.map(d=>`<option value="${d.employee}">${d.employee}</option>`).sort().join('');

    if (prevEmp && [...empFilter.options].some(o => o.value === prevEmp)) empFilter.value = prevEmp;

    renderMRConclusion(data);
    let all=[]; data.forEach(d=>{(d.mrs||[]).forEach(m=>{all.push({...m,employee:d.employee});});});

    // Apply existing filters
    const stateVal = document.getElementById('mrFilterState').value;
    const pipelineVal = document.getElementById('mrFilterPipeline').value;
    if (empFilter.value !== 'all') all = all.filter(m => m.employee === empFilter.value);
    if (stateVal !== 'all') all = all.filter(m => m.state === stateVal);
    if (pipelineVal !== 'all') all = all.filter(m => m.pipeline_status === pipelineVal);

    renderMRTable(all);
}
function filterMRs(){if(!mrData)return;let all=[];mrData.forEach(d=>{(d.mrs||[]).forEach(m=>{all.push({...m,employee:d.employee});});});const e=document.getElementById('mrFilterEmployee').value,s=document.getElementById('mrFilterState').value,p=document.getElementById('mrFilterPipeline').value;
    const search = (document.getElementById('mrSearch')?.value || '').toLowerCase();
    if(e!=='all')all=all.filter(m=>m.employee===e);if(s!=='all')all=all.filter(m=>m.state===s);if(p!=='all')all=all.filter(m=>m.pipeline_status===p);
    if(search) all=all.filter(m=>(m.title+' '+m.employee+' '+(m.project||'')+' '+(m.source_branch||'')).toLowerCase().includes(search));
    renderMRTable(all);}

let mrSortDir = 'desc'; // date sort direction for MR
function sortMRByDate() {
    mrSortDir = mrSortDir === 'desc' ? 'asc' : 'desc';
    document.getElementById('mrSortDate').textContent = mrSortDir === 'asc' ? '↑' : '↓';
    filterMRs();
}

function renderMRTable(mrs){const tb=document.getElementById('mrTableBody');if(!mrs||!mrs.length){tb.innerHTML='<tr><td colspan="9" style="text-align:center;padding:40px;color:var(--text-secondary)">Нет MR</td></tr>';return;}
// Sort by date
mrs.sort((a, b) => {
    const da = a.created_at || '', db = b.created_at || '';
    return mrSortDir === 'desc' ? db.localeCompare(da) : da.localeCompare(db);
});
// Group by employee, sorted alphabetically
const groups = {};
mrs.forEach(m => { if (!groups[m.employee]) groups[m.employee] = []; groups[m.employee].push(m); });
const sortedNames = Object.keys(groups).sort();
let html = '';
sortedNames.forEach(name => {
    const items = groups[name];
    const groupId = 'mr-' + name.replace(/[^a-zA-Zа-яА-ЯёЁ0-9]/g, '');
    html += `<tr class="group-header-row" onclick="toggleTableGroup('${groupId}')"><td colspan="9" class="group-header-cell"><i class="fas fa-chevron-right group-chevron" id="${groupId}-chev"></i><strong>${name}</strong><span class="group-count">${items.length} MR</span></td></tr>`;
    items.forEach(m => {
        const sl=m.state==='opened'?'Открыт':m.state==='merged'?'Merged':'Закрыт';const pl=pipLabel(m.pipeline_status);const pi=pipIcon(m.pipeline_status);const pc=m.pipeline_status||'pending';const dc=m.days_open>7?'critical':m.days_open>3?'warning':'ok';const rv=m.reviewers&&m.reviewers.length?`<div class="reviewers-list">${m.reviewers.map(r=>`<span class="reviewer-tag">${r}</span>`).join('')}</div>`:'<span class="no-reviewer">Нет</span>';
        const createdDate = m.created_at ? new Date(m.created_at).toLocaleDateString('ru-RU') : '-';
        html += `<tr class="group-row ${groupId}" style="display:none;"><td><a href="${m.url}" target="_blank" class="task-key">!${m.iid||m.id}</a></td><td style="font-size:12px">${m.employee}</td><td style="max-width:220px;overflow:hidden;text-overflow:ellipsis;white-space:nowrap">${m.title}</td><td style="font-size:11px;color:var(--text-secondary)">${m.project||m.source_branch}</td><td><span class="mr-state-badge ${m.state}">${sl}</span></td><td><span class="pipeline-badge ${pc}"><i class="fas ${pi}"></i> ${pl}</span></td><td style="font-size:12px">${createdDate}</td><td><span class="days-badge ${dc}">${m.days_open}д</span></td><td>${rv}</td></tr>`;
    });
});
tb.innerHTML = html;}
function renderMRConclusion(data){const el=document.getElementById('mrConclusion');const items=data.filter(d=>d.conclusion&&!d.conclusion.startsWith('Всё в порядке'));
if(!items.length){el.style.display='block';el.className='conclusion-banner';el.innerHTML=`<div class="conclusion-title"><i class="fas fa-check-circle"></i> Заключение</div><div class="conclusion-items"><div class="conclusion-item"><span class="issue-text" style="color:var(--accent-green)">MR обрабатываются нормально.</span></div></div>`;return;}
el.style.display='block';el.className='conclusion-banner has-issues collapsed';el.innerHTML=`<div class="conclusion-title" onclick="this.parentElement.classList.toggle('collapsed')"><i class="fas fa-exclamation-circle"></i> Заключение <span class="conclusion-count">${items.length}</span><i class="fas fa-chevron-down conclusion-chevron"></i></div><div class="conclusion-items">${data.map(d=>d.conclusion&&!d.conclusion.startsWith('Всё в порядке')?`<div class="conclusion-item"><span class="emp-name">${d.employee}</span><span class="issue-text">${d.conclusion.replace('Обратить внимание: ','')}</span></div>`:'').filter(Boolean).join('')}</div>`;}
function pipLabel(s){return{success:'Успешно',failed:'Ошибка',running:'Запущен',pending:'Ожидает'}[s]||s||'Н/Д';}
function pipIcon(s){return{success:'fa-check-circle',failed:'fa-times-circle',running:'fa-spinner',pending:'fa-clock'}[s]||'fa-question-circle';}

// === CONFLUENCE ===
async function loadConfluence() {
    initMonthFilter('confMonthFilter', () => loadConfluence());
    const month = document.getElementById('confMonthFilter')?.value || '';
    const url = month ? `${API_BASE}/api/confluence?month=${month}` : `${API_BASE}/api/confluence`;
    try { const r = await fetch(url); confData = await r.json(); renderConfPage(confData); } catch(e) { console.error(e); }
}
function renderConfPage(data) {
    const empFilter = document.getElementById('confFilterEmployee');
    const spaceFilter = document.getElementById('confFilterSpace');
    const prevEmp = empFilter.value;
    const prevSpace = spaceFilter.value;

    empFilter.innerHTML='<option value="all">Все сотрудники</option>'+data.map(d=>`<option value="${d.employee}">${d.employee}</option>`).sort().join('');
    const spaces=new Set(); data.forEach(d=>{(d.pages||[]).forEach(p=>{if(p.space)spaces.add(p.space+'|'+(p.space_name||p.space));});});
    spaceFilter.innerHTML='<option value="all">Все пространства</option>'+[...spaces].map(s=>{const[k,n]=s.split('|');return`<option value="${k}">${k} — ${n}</option>`;}).join('');

    if (prevEmp && [...empFilter.options].some(o => o.value === prevEmp)) empFilter.value = prevEmp;
    if (prevSpace && [...spaceFilter.options].some(o => o.value === prevSpace)) spaceFilter.value = prevSpace;

    renderConfConclusion(data);
    let all=[]; data.forEach(d=>{(d.pages||[]).forEach(p=>{all.push({...p,employee:d.employee});});});

    // Apply existing filters
    if (empFilter.value !== 'all') all = all.filter(p => p.employee === empFilter.value);
    if (spaceFilter.value !== 'all') all = all.filter(p => p.space === spaceFilter.value);

    renderConfTable(all);
}
function filterConfluence(){if(!confData)return;let all=[];confData.forEach(d=>{(d.pages||[]).forEach(p=>{all.push({...p,employee:d.employee});});});const e=document.getElementById('confFilterEmployee').value,s=document.getElementById('confFilterSpace').value;
    const search = (document.getElementById('confSearch')?.value || '').toLowerCase();
    if(e!=='all')all=all.filter(p=>p.employee===e);if(s!=='all')all=all.filter(p=>p.space===s);
    if(search) all=all.filter(p=>(p.title+' '+p.employee+' '+p.space+' '+(p.space_name||'')+' '+(p.changes||'')).toLowerCase().includes(search));
    renderConfTable(all);}
function renderConfTable(pages){const tb=document.getElementById('confTableBody');if(!pages||!pages.length){tb.innerHTML='<tr><td colspan="6" style="text-align:center;padding:40px;color:var(--text-secondary)">Нет данных</td></tr>';return;}
pages.sort((a,b)=>new Date(b.last_updated)-new Date(a.last_updated));
tb.innerHTML=pages.map(p=>{const dt=p.last_updated?new Date(p.last_updated).toLocaleDateString('ru-RU'):'-';
return`<tr><td><a href="${p.url}" target="_blank" class="task-key" style="max-width:200px;display:inline-block;overflow:hidden;text-overflow:ellipsis;white-space:nowrap">${p.title}</a></td><td style="font-size:12px">${p.employee}</td><td><span style="font-size:11px;background:var(--bg-secondary);padding:2px 6px;border-radius:4px">${p.space}</span></td><td style="text-align:center">v${p.version||1}</td><td>${dt}</td><td style="font-size:12px;color:var(--text-secondary);max-width:240px">${p.changes||'-'}</td></tr>`;}).join('');}
function renderConfConclusion(data){const el=document.getElementById('confConclusion');if(!el)return;const items=data.filter(d=>d.conclusion&&!d.conclusion.startsWith('Документация ведётся'));
if(!items.length){el.style.display='block';el.className='conclusion-banner';el.innerHTML=`<div class="conclusion-title"><i class="fas fa-check-circle"></i> Заключение</div><div class="conclusion-items"><div class="conclusion-item"><span class="issue-text" style="color:var(--accent-green)">Документация ведётся активно.</span></div></div>`;return;}
el.style.display='block';el.className='conclusion-banner has-issues collapsed';el.innerHTML=`<div class="conclusion-title" onclick="this.parentElement.classList.toggle('collapsed')"><i class="fas fa-exclamation-circle"></i> Заключение <span class="conclusion-count">${items.length}</span><i class="fas fa-chevron-down conclusion-chevron"></i></div><div class="conclusion-items">${data.map(d=>d.conclusion&&!d.conclusion.startsWith('Документация ведётся')?`<div class="conclusion-item"><span class="emp-name">${d.employee}</span><span class="issue-text">${d.conclusion.replace('Обратить внимание: ','')}</span></div>`:'').filter(Boolean).join('')}</div>`;}

// === HELPERS ===
function getInitials(n){const p=n.split(' ');return p.length>=2?(p[0][0]+p[1][0]).toUpperCase():n.substring(0,2).toUpperCase();}
function getDays(d){if(!d)return 0;return Math.floor((new Date()-new Date(d))/(86400000));}
function updateConnectionStatus(c){const el=document.getElementById('connectionStatus');if(c){el.innerHTML='<span class="status-dot"></span><span>Live</span>';el.style.color='var(--accent-green)';}else{el.innerHTML='<span class="status-dot" style="background:var(--accent-red)"></span><span>Offline</span>';el.style.color='var(--accent-red)';}}
function toggleAlerts(){const l=document.getElementById('alertsList'),i=document.getElementById('alertsToggleIcon');if(l.style.display==='none'){l.style.display='flex';i.className='fas fa-chevron-down';}else{l.style.display='none';i.className='fas fa-chevron-right';}}

// === EXPORT ===
function exportAlertsToExcel() {
    if (!dashboardData || !dashboardData.alerts || dashboardData.alerts.length === 0) {
        alert('Нет алертов для экспорта');
        return;
    }

    const rows = dashboardData.alerts.map(a => ({
        'Сотрудник': a.employee,
        'Тип': getAlertTypeLabel(a.type),
        'Серьёзность': a.severity === 'critical' ? 'Критичный' : 'Внимание',
        'Описание': a.message,
        'Задача': a.task_key || '-',
        'Ссылка': a.task_url || '-',
        'Дней в статусе': a.days_in_status || '-',
        'Дата': new Date(a.created_at).toLocaleDateString('ru-RU')
    }));

    const ws = XLSX.utils.json_to_sheet(rows);

    // Auto-width columns
    const colWidths = Object.keys(rows[0]).map(key => ({
        wch: Math.max(key.length, ...rows.map(r => String(r[key]).length)) + 2
    }));
    ws['!cols'] = colWidths;

    const wb = XLSX.utils.book_new();
    XLSX.utils.book_append_sheet(wb, ws, 'Алерты');

    const month = document.getElementById('dashboardMonthFilter')?.value || 'текущий';
    const filename = `alerts_${month || 'current'}_${new Date().toISOString().slice(0,10)}.xlsx`;
    XLSX.writeFile(wb, filename);
}

function getAlertTypeLabel(type) {
    switch(type) {
        case 'stale_task': return 'Задача зависла';
        case 'no_activity': return 'Нет активности';
        case 'mr_no_review': return 'MR без ревью';
        case 'overdue': return 'Просрочена';
        default: return type;
    }
}

// === CANDIDATES ===
let candidatesData = null;
let candidateTypeFilter = 'all';
let currentCandidateFormType = 'automation';

const COMPETENCIES_AUTOMATION = [
    // 7-балльная шкала
    {name: "Техники тестирования и тест-дизайна", max: 7},
    {name: "Классификация и виды тестирования", max: 7},
    {name: "Тестовая документация", max: 7},
    {name: "SQL / Работа с БД", max: 7},
    {name: "Клиент-сервер", max: 7},
    {name: "STLC / SDLC", max: 7},
    {name: "Работа с логами", max: 7},
    {name: "Test Strategy Basics", max: 7},
    {name: "Test Management Basics", max: 7},
    {name: "Эвристические подходы", max: 7},
    {name: "Python Core", max: 7},
    {name: "UI Automation (Selenium)", max: 7},
    {name: "API Automation", max: 7},
    {name: "HTTP / REST", max: 7},
    {name: "Page Object Pattern", max: 7},
    {name: "CI/CD", max: 7},
    {name: "Allure Reports", max: 7},
    {name: "Docker", max: 7},
    {name: "Performance Testing (k6)", max: 7},
    {name: "Управление командой", max: 7},
    {name: "Mentoring / Onboarding", max: 7},
    {name: "Code Review", max: 7},
    {name: "Проведение интервью", max: 7},
    {name: "Конфликтные ситуации", max: 7},
    {name: "Коммуникация с разработкой", max: 7},
    {name: "Приоритизация задач", max: 7},
    {name: "Flaky tests", max: 7},
    {name: "Regression Planning", max: 7},
    {name: "Automation Framework Design", max: 7},
    {name: "CI/CD Pipeline Scenario", max: 7},
    {name: "Bug prioritization", max: 7},
    // 3-балльная шкала
    {name: "JIRA", max: 3},
    {name: "Confluence", max: 3},
    {name: "Postman", max: 3},
    {name: "TestRail / TestOps", max: 3},
    {name: "Kibana / Graylog", max: 3},
    {name: "Git", max: 3},
    // 4-балльная шкала
    {name: "Коммуникация", max: 4},
    {name: "Системное мышление", max: 4},
    {name: "Самостоятельность", max: 4},
    {name: "Стрессоустойчивость", max: 4},
];

const COMPETENCIES_MANUAL = [
    // 7-балльная шкала
    {name: "Техники тестирования и тест-дизайна", max: 7},
    {name: "Классификация и виды тестирования", max: 7},
    {name: "Тестовая документация (тест-кейсы, чек-листы, баг-репорты)", max: 7},
    {name: "STLC / SDLC", max: 7},
    {name: "Test Strategy Basics", max: 7},
    {name: "Test Management Basics", max: 7},
    {name: "Эвристические подходы", max: 7},
    {name: "Exploratory Testing", max: 7},
    {name: "Risk-Based Testing", max: 7},
    {name: "SQL / Работа с БД", max: 7},
    {name: "Клиент-сервер", max: 7},
    {name: "HTTP / REST (понимание запросов, статус-кодов)", max: 7},
    {name: "Работа с логами", max: 7},
    {name: "Работа с DevTools (Chrome/Safari)", max: 7},
    {name: "Postman (базовая проверка API)", max: 7},
    {name: "Мобильное тестирование (Android/iOS)", max: 7},
    {name: "Кросс-браузерное / кросс-платформенное тестирование", max: 7},
    {name: "Локализация и интернационализация", max: 7},
    {name: "Accessibility Testing (WCAG)", max: 7},
    {name: "Управление командой", max: 7},
    {name: "Mentoring / Onboarding", max: 7},
    {name: "Проведение интервью", max: 7},
    {name: "Конфликтные ситуации", max: 7},
    {name: "Коммуникация с разработкой", max: 7},
    {name: "Приоритизация задач", max: 7},
    {name: "Планирование регрессии", max: 7},
    {name: "Управление тестовыми данными", max: 7},
    {name: "Координация релизов", max: 7},
    {name: "Bug prioritization / severity", max: 7},
    {name: "Regression Planning", max: 7},
    {name: "Test Coverage Analysis", max: 7},
    {name: "Release Readiness Assessment", max: 7},
    {name: "Воспроизведение сложных багов", max: 7},
    {name: "Тестирование интеграций", max: 7},
    {name: "Smoke / Sanity стратегия", max: 7},
    // 3-балльная шкала
    {name: "JIRA", max: 3},
    {name: "Confluence", max: 3},
    {name: "TestRail / TestOps", max: 3},
    {name: "Kibana / Graylog", max: 3},
    {name: "Git (базовый уровень)", max: 3},
    {name: "Charles / Fiddler (сниффинг трафика)", max: 3},
    {name: "Figma (сверка с макетами)", max: 3},
    {name: "BrowserStack / устройства", max: 3},
    // 4-балльная шкала
    {name: "Коммуникация", max: 4},
    {name: "Системное мышление", max: 4},
    {name: "Внимательность к деталям", max: 4},
    {name: "Самостоятельность", max: 4},
    {name: "Стрессоустойчивость", max: 4},
    {name: "Аналитическое мышление", max: 4},
];

// Legacy compatibility
const COMPETENCIES = COMPETENCIES_AUTOMATION.map(c => c.name);

function getCompetenciesForType(type) {
    return type === 'manual' ? COMPETENCIES_MANUAL : COMPETENCIES_AUTOMATION;
}

function filterCandidatesByType(type) {
    candidateTypeFilter = type;
    document.querySelectorAll('#candidatesTypeFilter .btn-toggle').forEach(btn => {
        btn.classList.toggle('active', btn.dataset.type === type);
    });
    filterCandidates();
}

function switchCandidateType(type) {
    currentCandidateFormType = type;
    document.querySelectorAll('#cf_type_toggle .btn-toggle').forEach(btn => {
        btn.classList.toggle('active', btn.dataset.type === type);
    });
    renderCompetencyFields(type);
}

function renderCompetencyFields(type, existingScores) {
    const comps = getCompetenciesForType(type);
    const container = document.getElementById('cf_competencies');
    container.innerHTML = comps.map((comp, i) => {
        const existing = existingScores ? existingScores[i] : null;
        const score = existing ? existing.score : 1;
        const comment = existing ? existing.comment : '';
        return `
        <div class="form-comp-row">
            <span class="form-comp-name">${comp.name}</span>
            <input type="number" min="1" max="${comp.max}" value="${score}" id="cf_score_${i}" class="form-input-sm" title="Макс: ${comp.max}">
            <input type="text" placeholder="Комментарий" value="${comment}" id="cf_comment_${i}" class="form-input" style="flex:1">
        </div>`;
    }).join('');
}

async function loadCandidates() {
    initMonthFilter('candidatesMonthFilter', () => loadCandidates());
    const month = document.getElementById('candidatesMonthFilter')?.value || '';
    const url = month ? `${API_BASE}/api/candidates?month=${month}` : `${API_BASE}/api/candidates`;
    try { const r = await fetch(url, {cache:'no-store'}); candidatesData = await r.json(); renderCandidatesPage(candidatesData); } catch(e) { console.error(e); }
}

function renderCandidatesPage(data) {
    // Stats
    const s = data.stats;
    document.getElementById('candidatesStats').innerHTML = `
        <div class="summary-card"><div class="summary-icon blue"><i class="fas fa-users"></i></div><div class="summary-content"><span class="summary-value">${s.total}</span><span class="summary-label">Всего</span></div></div>
        <div class="summary-card"><div class="summary-icon green"><i class="fas fa-user-check"></i></div><div class="summary-content"><span class="summary-value">${s.accepted}</span><span class="summary-label">Принято</span></div></div>
        <div class="summary-card"><div class="summary-icon red"><i class="fas fa-user-times"></i></div><div class="summary-content"><span class="summary-value">${s.rejected}</span><span class="summary-label">Отклонено</span></div></div>
        <div class="summary-card"><div class="summary-icon orange"><i class="fas fa-percentage"></i></div><div class="summary-content"><span class="summary-value">${s.conversion}%</span><span class="summary-label">Конверсия</span></div></div>
        <div class="summary-card"><div class="summary-icon purple"><i class="fas fa-star-half-alt"></i></div><div class="summary-content"><span class="summary-value">${s.avg_score}</span><span class="summary-label">Средний балл</span></div></div>
    `;
    // Conclusion
    const el = document.getElementById('candidatesConclusion');
    el.style.display = 'block';
    const hasIssues = s.conversion < 30 || s.total === 0;
    el.className = `conclusion-banner ${hasIssues ? 'has-issues' : ''}`;
    el.innerHTML = `<div class="conclusion-title"><i class="fas ${hasIssues?'fa-exclamation-circle':'fa-check-circle'}"></i> Заключение</div><div class="conclusion-items"><div class="conclusion-item"><span class="issue-text">${data.conclusion}</span></div></div>`;
    // Table
    let candidates = data.candidates || [];
    const resultFilter = document.getElementById('candidatesResultFilter').value;
    if (resultFilter !== 'all') candidates = candidates.filter(c => c.result === resultFilter);
    renderCandidatesTable(candidates);
}

function filterCandidates() {
    if (!candidatesData) return;
    let candidates = candidatesData.candidates || [];
    const resultFilter = document.getElementById('candidatesResultFilter').value;
    const search = (document.getElementById('candidatesSearch')?.value || '').toLowerCase();
    if (resultFilter !== 'all') candidates = candidates.filter(c => c.result === resultFilter);
    if (candidateTypeFilter !== 'all') candidates = candidates.filter(c => c.type === candidateTypeFilter);
    if (search) candidates = candidates.filter(c => (c.name + ' ' + c.conclusion + ' ' + c.level + ' ' + c.result).toLowerCase().includes(search));
    renderCandidatesTable(candidates);
}

function renderCandidatesTable(candidates) {
    const tb = document.getElementById('candidatesTableBody');
    if (!candidates || !candidates.length) { tb.innerHTML = '<tr><td colspan="8" style="text-align:center;padding:40px;color:var(--text-secondary)">Нет данных</td></tr>'; return; }
    tb.innerHTML = candidates.map(c => {
        const date = new Date(c.date).toLocaleDateString('ru-RU');
        const resultLabel = c.result === 'accepted' ? 'Принят' : c.result === 'accepted_no_sb' ? 'Не прошёл СБ' : c.result === 'pending' ? 'В ожидании' : 'Отклонён';
        const resultClass = c.result === 'accepted' ? 'done' : c.result === 'accepted_no_sb' ? 'review' : c.result === 'pending' ? 'status-blue' : 'stale';
        const typeLabel = c.type === 'manual' ? 'Manual' : 'Automation';
        const typeClass = c.type === 'manual' ? 'status-orange' : 'status-purple';
        const scores = c.competencies ? c.competencies.map(comp => `<span title="${comp.name}: ${comp.comment||''}" style="display:inline-block;width:18px;height:18px;line-height:18px;text-align:center;border-radius:3px;font-size:10px;margin-right:2px;background:${comp.score>=4?'rgba(52,211,153,0.2)':comp.score>=3?'rgba(251,191,36,0.2)':'rgba(248,113,113,0.2)'};color:${comp.score>=4?'var(--accent-green)':comp.score>=3?'var(--accent-yellow)':'var(--accent-red)'}">${comp.score}</span>`).join('') : '';
        return `<tr>
            <td><strong style="font-size:13px">${c.name}</strong><div style="margin-top:4px">${scores}</div></td>
            <td><span class="task-status-badge ${typeClass}" style="font-size:11px">${typeLabel}</span></td>
            <td style="font-size:12px">${date}</td>
            <td style="font-size:14px;font-weight:600">${c.avg_score}</td>
            <td><span class="task-status-badge in-progress">${c.level}</span></td>
            <td><span class="task-status-badge ${resultClass}">${resultLabel}</span></td>
            <td><span class="editable-conclusion" onclick="editConclusion('${c.id}', this)" title="Нажмите для редактирования">${c.conclusion || '—'}</span></td>
            <td>
                <div style="display:flex;gap:4px">
                    <button class="btn-icon" onclick="editCandidate('${c.id}')" title="Редактировать"><i class="fas fa-pen"></i></button>
                    <button class="btn-icon btn-icon-danger" onclick="deleteCandidate('${c.id}','${c.name}')" title="Удалить"><i class="fas fa-trash"></i></button>
                </div>
            </td>
        </tr>`;
    }).join('');
}

function showAddCandidateForm() {
    document.getElementById('candidateFormOverlay').style.display = 'flex';
    document.getElementById('cf_name').value = '';
    document.getElementById('cf_date').value = new Date().toISOString().slice(0, 10);
    document.getElementById('cf_result').value = 'rejected';
    document.getElementById('cf_conclusion').value = '';
    document.getElementById('cf_name').dataset.editId = '';
    currentCandidateFormType = 'automation';
    document.querySelectorAll('#cf_type_toggle .btn-toggle').forEach(btn => {
        btn.classList.toggle('active', btn.dataset.type === 'automation');
    });
    renderCompetencyFields('automation');
}

function editCandidate(id) {
    const candidates = candidatesData?.candidates || [];
    const c = candidates.find(x => x.id === id);
    if (!c) return;

    document.getElementById('candidateFormOverlay').style.display = 'flex';
    document.getElementById('cf_name').value = c.name;
    document.getElementById('cf_name').dataset.editId = c.id;
    document.getElementById('cf_date').value = c.date ? c.date.slice(0, 10) : '';
    document.getElementById('cf_result').value = c.result || 'rejected';
    document.getElementById('cf_conclusion').value = c.conclusion || '';

    const type = c.type || 'automation';
    currentCandidateFormType = type;
    document.querySelectorAll('#cf_type_toggle .btn-toggle').forEach(btn => {
        btn.classList.toggle('active', btn.dataset.type === type);
    });
    renderCompetencyFields(type, c.competencies);
}

function hideAddCandidateForm() { document.getElementById('candidateFormOverlay').style.display = 'none'; }

async function submitCandidate() {
    const name = document.getElementById('cf_name').value.trim();
    if (!name) { alert('Укажите ФИО'); return; }
    const editId = document.getElementById('cf_name').dataset.editId;
    const date = document.getElementById('cf_date').value;
    const result = document.getElementById('cf_result').value;
    const conclusion = document.getElementById('cf_conclusion').value.trim();
    const comps = getCompetenciesForType(currentCandidateFormType);
    const competencies = comps.map((comp, i) => ({
        name: comp.name,
        score: parseInt(document.getElementById(`cf_score_${i}`).value) || 1,
        comment: document.getElementById(`cf_comment_${i}`).value.trim()
    }));
    const body = { name, date: date + 'T00:00:00+05:00', type: currentCandidateFormType, result, conclusion, competencies };

    let url = `${API_BASE}/api/candidates/add`;
    if (editId) {
        body.id = editId;
        url = `${API_BASE}/api/candidates/update`;
    }

    try {
        const r = await fetch(url, { method: 'POST', headers: {'Content-Type':'application/json'}, body: JSON.stringify(body) });
        if (r.ok) { hideAddCandidateForm(); loadCandidates(); } else { alert('Ошибка сохранения'); }
    } catch(e) { alert('Ошибка: ' + e.message); }
}

function exportCandidatesToExcel() {
    if (!candidatesData || !candidatesData.candidates || !candidatesData.candidates.length) { alert('Нет данных'); return; }
    const rows = candidatesData.candidates.map(c => {
        const row = { 'ФИО': c.name, 'Дата': new Date(c.date).toLocaleDateString('ru-RU'), 'Средний балл': c.avg_score, 'Уровень': c.level, 'Grade': c.grade, 'Результат': c.result === 'accepted' ? 'Принят' : c.result === 'accepted_no_sb' ? 'Не прошёл СБ' : c.result === 'pending' ? 'В ожидании' : 'Отклонён', 'Заключение': c.conclusion };
        (c.competencies || []).forEach(comp => { row[comp.name] = comp.score; });
        return row;
    });
    const ws = XLSX.utils.json_to_sheet(rows);
    ws['!cols'] = Object.keys(rows[0]).map(k => ({ wch: Math.max(k.length, 12) }));
    const wb = XLSX.utils.book_new();
    XLSX.utils.book_append_sheet(wb, ws, 'Собеседования');
    XLSX.writeFile(wb, `interviews_${new Date().toISOString().slice(0,10)}.xlsx`);
}

async function deleteCandidate(id, name) {
    if (!confirm(`Удалить кандидата "${name}"?`)) return;
    try {
        const r = await fetch(`${API_BASE}/api/candidates/delete`, { method: 'POST', headers: {'Content-Type':'application/json'}, body: JSON.stringify({id}) });
        if (r.ok) loadCandidates();
        else alert('Ошибка удаления');
    } catch(e) { alert('Ошибка: ' + e.message); }
}

function editConclusion(id, el) {
    const current = el.textContent;
    const input = document.createElement('textarea');
    input.value = current === '—' ? '' : current;
    input.className = 'form-input';
    input.style.fontSize = '12px';
    input.style.minHeight = '50px';
    input.style.width = '100%';
    el.replaceWith(input);
    input.focus();

    async function save() {
        const newText = input.value.trim();
        try {
            await fetch(`${API_BASE}/api/candidates/conclusion`, { method: 'POST', headers: {'Content-Type':'application/json'}, body: JSON.stringify({id, conclusion: newText}) });
        } catch(e) {}
        loadCandidates();
    }

    input.addEventListener('blur', save);
    input.addEventListener('keydown', (e) => { if (e.key === 'Enter' && !e.shiftKey) { e.preventDefault(); save(); } });
}

// === VPN / ATTENDANCE ===
let vpnData = [];
const VPN_WS = 510, VPN_WE = 1200; // 8:30 and 20:00 in minutes

function loadAttendance() {
    // Generate month options
    const sel = document.getElementById('vpnPeriodFilter');
    if (!sel.dataset.init) {
        sel.dataset.init = '1';
        const now = new Date();
        // Start from previous month (current month may not have full data yet)
        for (let i = 1; i < 12; i++) {
            const d = new Date(now.getFullYear(), now.getMonth() - i, 1);
            const year = d.getFullYear();
            const month = String(d.getMonth() + 1).padStart(2, '0');
            const val = `${year}-${month}`;
            const label = d.toLocaleDateString('ru', { month: 'long', year: 'numeric' });
            const o = document.createElement('option');
            o.value = val; o.textContent = label;
            sel.appendChild(o);
        }
    }
    fetchVPNData();
}

async function fetchVPNData() {
    try {
        const res = await fetch(`${API_BASE}/api/vpn/data`);
        vpnData = await res.json();
        populateVPNFilter();
        renderVPN();
    } catch (e) { console.error('VPN fetch error:', e); }
}

function populateVPNFilter() {
    const sel = document.getElementById('vpnUserFilter');
    const cur = sel.value;
    const users = [...new Set(vpnData.map(r => r.name))].sort();
    sel.innerHTML = '<option value="">Все сотрудники</option>' + users.map(u => `<option value="${u}">${u}</option>`).join('');
    sel.value = cur;
}

function vpnToMin(t) { const [h, m] = t.split(':').map(Number); return h * 60 + m; }
function vpnToTime(m) { m = ((m % 1440) + 1440) % 1440; return String(Math.floor(m / 60)).padStart(2, '0') + ':' + String(m % 60).padStart(2, '0'); }
function vpnDiffH(s, e) { let d = e - s; if (d < 0) d += 1440; return Math.round(d / 6) / 10; }

let vpnSortKey = 'date';
let vpnSortDir = 'desc';

function sortVPN(key) {
    if (vpnSortKey === key) {
        vpnSortDir = vpnSortDir === 'desc' ? 'asc' : 'desc';
    } else {
        vpnSortKey = key;
        vpnSortDir = key === 'name' ? 'asc' : 'desc';
    }
    // Update icons
    ['name', 'date', 'total'].forEach(k => {
        const el = document.getElementById('vpnSort-' + k);
        if (el) el.textContent = k === vpnSortKey ? (vpnSortDir === 'asc' ? '↑' : '↓') : '↕';
    });
    renderVPN();
}

function renderVPN() {
    const userF = document.getElementById('vpnUserFilter').value;
    const periodF = document.getElementById('vpnPeriodFilter').value;
    const searchF = document.getElementById('vpnSearch').value.toLowerCase().trim();

    let filtered = vpnData;
    if (periodF === 'current') {
        const ym = new Date().toISOString().slice(0, 7);
        filtered = filtered.filter(r => r.date.startsWith(ym));
    } else if (periodF !== 'all') {
        filtered = filtered.filter(r => r.date.startsWith(periodF));
    }
    if (userF) filtered = filtered.filter(r => r.name === userF);
    if (searchF) filtered = filtered.filter(r => r.name.toLowerCase().includes(searchF));

    const tb = document.getElementById('vpnTableBody');
    if (!filtered.length) {
        tb.innerHTML = '<tr><td colspan="7" style="text-align:center;padding:40px;color:var(--text-secondary)">Нет данных. Загрузите xlsx файл.</td></tr>';
        document.getElementById('vpnStats').innerHTML = '';
        return;
    }

    const rows = filtered.map(r => {
        if (r.flag === 'absent' || r.flag === 'weekend_off' || r.flag === 'holiday_off') return { ...r, workP: '—', workH: 0, afterP: '—', afterH: 0 };
        const sM = vpnToMin(r.start), eRaw = vpnToMin(r.end); let eM = eRaw; if (eM <= sM) eM += 1440;
        let workP = '—', workH = 0, afterP = '—', afterH = 0; const ap = [];
        if (r.flag === 'weekend_work' || r.flag === 'holiday_work') {
            afterP = vpnToTime(sM) + '–' + vpnToTime(eM); afterH = vpnDiffH(sM, eM); return { ...r, workP, workH, afterP, afterH };
        }
        if (sM < VPN_WS) { const se = Math.min(eM, VPN_WS); if (se > sM) { ap.push(vpnToTime(sM) + '–' + vpnToTime(se)); afterH += vpnDiffH(sM, se); } }
        const ws = Math.max(sM, VPN_WS), we = Math.min(eM, VPN_WE); if (we > ws) { workP = vpnToTime(ws) + '–' + vpnToTime(we); workH = vpnDiffH(ws, we); }
        if (eM > VPN_WE) { const as = Math.max(sM, VPN_WE); ap.push(vpnToTime(as) + '–' + vpnToTime(eM)); afterH += vpnDiffH(as, eM); }
        if (ap.length) afterP = ap.join(', ');
        return { ...r, workP, workH, afterP, afterH };
    });

    // Sort by selected column
    rows.sort((a, b) => {
        let cmp = 0;
        if (vpnSortKey === 'date') cmp = (a.date || '').localeCompare(b.date || '');
        else if (vpnSortKey === 'name') cmp = (a.name || '').localeCompare(b.name || '');
        else if (vpnSortKey === 'total') cmp = (a.workH + a.afterH) - (b.workH + b.afterH);
        return vpnSortDir === 'asc' ? cmp : -cmp;
    });

    // Stats
    const totalWork = rows.reduce((s, r) => s + r.workH, 0).toFixed(1);
    const totalAfter = rows.reduce((s, r) => s + r.afterH, 0).toFixed(1);
    const uniqueU = new Set(rows.map(r => r.name)).size;
    const absentDays = rows.filter(r => r.flag === 'absent').length;
    const weekendDays = rows.filter(r => r.flag === 'weekend_work' || r.flag === 'holiday_work').length;

    document.getElementById('vpnStats').innerHTML = `
        <div class="summary-card"><div class="summary-icon blue"><i class="fas fa-users"></i></div><div class="summary-content"><span class="summary-value">${uniqueU}</span><span class="summary-label">Сотрудников</span></div></div>
        <div class="summary-card"><div class="summary-icon green"><i class="fas fa-clock"></i></div><div class="summary-content"><span class="summary-value">${totalWork} ч</span><span class="summary-label">В раб. время</span></div></div>
        <div class="summary-card"><div class="summary-icon red"><i class="fas fa-moon"></i></div><div class="summary-content"><span class="summary-value">${totalAfter} ч</span><span class="summary-label">Вне раб. времени</span></div></div>
        <div class="summary-card"><div class="summary-icon orange"><i class="fas fa-exclamation-triangle"></i></div><div class="summary-content"><span class="summary-value">${absentDays}</span><span class="summary-label">Пропущено</span></div></div>
        <div class="summary-card"><div class="summary-icon green"><i class="fas fa-calendar-check"></i></div><div class="summary-content"><span class="summary-value">${weekendDays}</span><span class="summary-label">В выходные</span></div></div>
        <div class="summary-card"><div class="summary-icon blue"><i class="fas fa-list"></i></div><div class="summary-content"><span class="summary-value">${rows.length}</span><span class="summary-label">Записей</span></div></div>
    `;

    const flagTag = { 'normal': '', 'absent': '<span class="task-status-badge" style="background:rgba(251,146,60,.12);color:var(--accent-orange)">Нет подключения</span>', 'weekend_work': '<span class="task-status-badge" style="background:rgba(52,211,153,.12);color:var(--accent-green)">Выходной (работал)</span>', 'holiday_work': '<span class="task-status-badge" style="background:rgba(167,139,250,.12);color:var(--accent-purple)">Праздник (работал)</span>', 'weekend_off': '<span class="task-status-badge" style="background:var(--bg-secondary);color:var(--text-muted)">Выходной</span>', 'holiday_off': '<span class="task-status-badge" style="background:var(--bg-secondary);color:var(--text-muted)">Праздник</span>' };

    tb.innerHTML = rows.map(r => {
        const isOff = r.flag === 'weekend_off' || r.flag === 'holiday_off';
        const totalH = Math.round((r.workH + r.afterH) * 10) / 10;
        return `<tr${r.flag === 'absent' ? ' style="background:rgba(251,146,60,.04)"' : ''}>
            <td>${r.name}</td>
            <td style="font-size:12px">${r.date.split('-').reverse().join('.')}</td>
            <td style="color:var(--accent-green);font-weight:600">${isOff ? '' : r.workP}</td>
            <td style="color:var(--accent-green);font-weight:600">${isOff ? '' : (r.workH ? r.workH + ' ч' : '—')}</td>
            <td>${isOff ? '' : (r.afterH ? '<span style="color:var(--accent-red);font-weight:600">' + r.afterP + '</span>' : '—')}</td>
            <td>${isOff ? '' : (r.afterH ? '<span style="color:var(--accent-red);font-weight:600">' + r.afterH + ' ч</span>' : '—')}</td>
            <td style="font-weight:700;color:var(--accent-blue)">${isOff ? '' : (totalH ? totalH + ' ч' : '—')}</td>
            <td>${flagTag[r.flag] || ''}</td>
        </tr>`;
    }).join('');
}

function openVPNUpload() { document.getElementById('vpnUploadOverlay').style.display = 'flex'; }
function closeVPNUpload() { document.getElementById('vpnUploadOverlay').style.display = 'none'; }

// VPN upload handlers
(function() {
    document.addEventListener('DOMContentLoaded', () => {
        const dropZone = document.getElementById('vpnDropZone');
        const fileInput = document.getElementById('vpnFileInput');
        if (!dropZone || !fileInput) return;
        dropZone.addEventListener('click', () => fileInput.click());
        dropZone.addEventListener('dragover', e => { e.preventDefault(); dropZone.style.borderColor = 'var(--accent-magenta)'; });
        dropZone.addEventListener('dragleave', () => { dropZone.style.borderColor = 'var(--border)'; });
        dropZone.addEventListener('drop', e => { e.preventDefault(); dropZone.style.borderColor = 'var(--border)'; if (e.dataTransfer.files.length) uploadVPNFile(e.dataTransfer.files[0]); });
        fileInput.addEventListener('change', () => { if (fileInput.files.length) uploadVPNFile(fileInput.files[0]); });
    });
})();

async function uploadVPNFile(file) {
    const msg = document.getElementById('vpnUploadMsg');
    msg.textContent = 'Загрузка...'; msg.style.color = 'var(--accent-blue)';
    const fd = new FormData(); fd.append('file', file);
    try {
        const res = await fetch(`${API_BASE}/api/vpn/upload`, { method: 'POST', body: fd });
        const data = await res.json();
        if (data.success) { msg.style.color = 'var(--accent-green)'; msg.textContent = '✅ ' + data.message; fetchVPNData(); }
        else { msg.style.color = 'var(--accent-red)'; msg.textContent = '❌ ' + data.error; }
    } catch (e) { msg.style.color = 'var(--accent-red)'; msg.textContent = '❌ Ошибка сети'; }
}

// === THEME ===
function toggleTheme() {
    const html = document.documentElement;
    const current = html.getAttribute('data-theme');
    const next = current === 'light' ? 'dark' : 'light';
    html.setAttribute('data-theme', next);
    localStorage.setItem('theme', next);
    updateThemeIcon(next);
}

function updateThemeIcon(theme) {
    const icon = document.getElementById('themeIcon');
    if (icon) {
        icon.textContent = theme === 'light' ? '☀️' : '🌙';
    }
}

// Apply saved theme on load
(function() {
    const saved = localStorage.getItem('theme') || 'dark';
    document.documentElement.setAttribute('data-theme', saved);
    document.addEventListener('DOMContentLoaded', () => updateThemeIcon(saved));
})();

// === TASKS EXPORT ===
function showTasksExportDialog() {
    const employees = tasksData ? tasksData.map(d => d.employee) : [];
    const month = document.getElementById('tasksMonthFilter')?.value || '';

    // Collect types and projects from current data
    const types = new Set();
    const projects = new Set();
    if (tasksData) {
        tasksData.forEach(d => {
            (d.issues || []).forEach(i => {
                types.add(i.type);
                projects.add(i.project);
            });
        });
    }

    const overlay = document.createElement('div');
    overlay.className = 'candidate-form-overlay';
    overlay.id = 'tasksExportOverlay';
    overlay.innerHTML = `
        <div class="candidate-form" style="width:480px;max-height:80vh;overflow-y:auto">
            <h3><i class="fas fa-file-excel" style="color:var(--accent-green)"></i> Экспорт задач в Excel</h3>
            <p style="font-size:13px;color:var(--text-secondary);margin-bottom:14px">Комментарии будут включены в выгрузку.</p>
            <div style="margin-bottom:14px">
                <label style="font-size:12px;color:var(--text-secondary)">Период:</label>
                <select class="form-input" id="exportMonth" style="margin-top:4px">
                    <option value="" ${!month?'selected':''}>Текущий месяц</option>
                    <option value="all">За весь период</option>
                    ${getMonthOptions().map(m => `<option value="${m.val}" ${m.val===month?'selected':''}>${m.label}</option>`).join('')}
                </select>
            </div>
            <div style="margin-bottom:14px">
                <label style="font-size:12px;color:var(--text-secondary)">Сотрудники:</label>
                <div style="margin-top:4px;margin-bottom:4px"><a href="#" onclick="toggleExportCheckboxes('.export-emp-cb', true);return false" style="font-size:11px;color:var(--accent-magenta);margin-right:10px">Выбрать все</a><a href="#" onclick="toggleExportCheckboxes('.export-emp-cb', false);return false" style="font-size:11px;color:var(--text-muted)">Сбросить</a></div>
                <div style="display:flex;flex-direction:column;gap:6px">
                    ${employees.map(e => `<label style="display:flex;align-items:center;gap:8px;font-size:13px;cursor:pointer"><input type="checkbox" class="export-emp-cb" value="${e}" checked> ${e}</label>`).join('')}
                </div>
            </div>
            <div style="margin-bottom:14px">
                <label style="font-size:12px;color:var(--text-secondary)">Типы задач:</label>
                <div style="margin-top:4px;margin-bottom:4px"><a href="#" onclick="toggleExportCheckboxes('.export-type-cb', true);return false" style="font-size:11px;color:var(--accent-magenta);margin-right:10px">Выбрать все</a><a href="#" onclick="toggleExportCheckboxes('.export-type-cb', false);return false" style="font-size:11px;color:var(--text-muted)">Сбросить</a></div>
                <div style="display:flex;flex-direction:column;gap:6px">
                    ${[...types].map(t => `<label style="display:flex;align-items:center;gap:8px;font-size:13px;cursor:pointer"><input type="checkbox" class="export-type-cb" value="${t}" checked> ${t}</label>`).join('')}
                </div>
            </div>
            <div style="margin-bottom:14px">
                <label style="font-size:12px;color:var(--text-secondary)">Проекты:</label>
                <div style="margin-top:4px;margin-bottom:4px"><a href="#" onclick="toggleExportCheckboxes('.export-project-cb', true);return false" style="font-size:11px;color:var(--accent-magenta);margin-right:10px">Выбрать все</a><a href="#" onclick="toggleExportCheckboxes('.export-project-cb', false);return false" style="font-size:11px;color:var(--text-muted)">Сбросить</a></div>
                <div style="display:flex;flex-direction:column;gap:6px;max-height:120px;overflow-y:auto">
                    ${[...projects].map(p => `<label style="display:flex;align-items:center;gap:8px;font-size:13px;cursor:pointer"><input type="checkbox" class="export-project-cb" value="${p}" checked> ${p}</label>`).join('')}
                </div>
            </div>
            <div class="form-actions">
                <button class="btn-save" onclick="executeTasksExport()"><i class="fas fa-download"></i> Выгрузить</button>
                <button class="btn-cancel" onclick="document.getElementById('tasksExportOverlay').remove()">Отмена</button>
            </div>
        </div>
    `;
    document.body.appendChild(overlay);
}

async function executeTasksExport() {
    const month = document.getElementById('exportMonth').value;
    const checkboxes = document.querySelectorAll('.export-emp-cb:checked');
    const employees = [...checkboxes].map(cb => cb.value).join(',');
    const selectedTypes = new Set([...document.querySelectorAll('.export-type-cb:checked')].map(cb => cb.value));
    const selectedProjects = new Set([...document.querySelectorAll('.export-project-cb:checked')].map(cb => cb.value));

    if (!employees) { alert('Выберите хотя бы одного сотрудника'); return; }

    const btn = document.querySelector('#tasksExportOverlay .btn-save');
    btn.innerHTML = '<i class="fas fa-spinner fa-spin"></i> Загрузка...';
    btn.disabled = true;

    try {
        const url = `${API_BASE}/api/tasks/export?month=${month}&employees=${encodeURIComponent(employees)}`;
        const resp = await fetch(url, { cache: 'no-store' });
        if (!resp.ok) throw new Error('Ошибка сервера');
        let data = await resp.json();

        // Filter by type and project on client side
        data = data.filter(task => selectedTypes.has(task.type) && selectedProjects.has(task.project));

        const rows = data.map(task => {
            const commentsText = (task.comments || [])
                .map(c => `[${c.created ? c.created.slice(0,10) : ''}] ${c.author}: ${c.body}`)
                .join('\n---\n');
            return {
                'Ключ': task.key,
                'Сотрудник': task.employee,
                'Задача': task.summary,
                'Тип': task.type,
                'Статус': task.status,
                'Проект': task.project,
                'Дата создания': task.created,
                'Дата обновления': task.updated,
                'Ссылка': task.url,
                'Комментарии': commentsText || '-'
            };
        });

        if (!rows.length) { alert('Нет задач для выгрузки'); btn.innerHTML = '<i class="fas fa-download"></i> Выгрузить'; btn.disabled = false; return; }

        const ws = XLSX.utils.json_to_sheet(rows);
        ws['!cols'] = Object.keys(rows[0]).map(k => ({ wch: k === 'Комментарии' ? 60 : k === 'Задача' ? 40 : k === 'Ссылка' ? 50 : 16 }));
        const wb = XLSX.utils.book_new();
        XLSX.utils.book_append_sheet(wb, ws, 'Задачи');
        const filename = `tasks_${month || 'current'}_${new Date().toISOString().slice(0,10)}.xlsx`;
        XLSX.writeFile(wb, filename);

        document.getElementById('tasksExportOverlay').remove();
    } catch(e) {
        alert('Ошибка: ' + e.message);
        btn.innerHTML = '<i class="fas fa-download"></i> Выгрузить';
        btn.disabled = false;
    }
}

// === TASK COMMENTS (by click) ===
async function loadTaskComments(key, btn) {
    const row = btn.closest('tr');
    // Check if already expanded
    const nextRow = row.nextElementSibling;
    if (nextRow && nextRow.classList.contains('comments-row')) {
        nextRow.remove();
        return;
    }
    // Show loading
    btn.innerHTML = '<i class="fas fa-spinner fa-spin"></i>';
    try {
        const commResp = await fetch(`${API_BASE}/api/tasks/comments?key=${key}`, { cache: 'no-store' });
        if (!commResp.ok) throw new Error('Failed');
        const comments = await commResp.json();
        btn.innerHTML = '<i class="fas fa-comment"></i>';

        const commRow = document.createElement('tr');
        commRow.className = 'comments-row';
        if (!comments || !comments.length) {
            commRow.innerHTML = `<td colspan="10" style="padding:12px 20px;font-size:12px;color:var(--text-muted);background:var(--bg-secondary)">Нет комментариев</td>`;
        } else {
            const html = comments.map(c => `<div style="margin-bottom:8px;padding:6px 0;border-bottom:1px solid var(--border)"><strong style="color:var(--accent-magenta)">${c.author}</strong> <span style="color:var(--text-muted);font-size:11px">${c.created?new Date(c.created).toLocaleDateString('ru-RU'):''}</span><div style="margin-top:4px;white-space:pre-wrap">${c.body}</div></div>`).join('');
            commRow.innerHTML = `<td colspan="10" style="padding:12px 20px;font-size:12px;background:var(--bg-secondary);max-height:200px;overflow-y:auto">${html}</td>`;
        }
        row.after(commRow);
    } catch(e) {
        btn.innerHTML = '<i class="fas fa-comment"></i>';
        alert('Ошибка загрузки комментариев');
    }
}

function toggleExportCheckboxes(selector, checked) {
    document.querySelectorAll(selector).forEach(cb => cb.checked = checked);
}


