// tgState Frontend JavaScript

const API_BASE = '';

// 通用函数
async function apiCall(url, options = {}) {
    const defaultOptions = {
        credentials: 'include',
        headers: {
            'Content-Type': 'application/json',
        },
    };
    
    const response = await fetch(url, { ...defaultOptions, ...options });
    const data = await response.json();
    
    if (response.status === 401) {
        window.location.href = '/login';
        throw new Error('Unauthorized');
    }
    
    return data;
}

// 登录
async function login(password) {
    const result = await apiCall('/api/auth/login', {
        method: 'POST',
        body: JSON.stringify({ password }),
    });
    
    if (result.code === 'success') {
        window.location.href = '/';
    } else {
        alert(result.message || '登录失败');
    }
}

// 登出
async function logout() {
    await apiCall('/api/auth/logout', {
        method: 'POST',
    });
    window.location.href = '/login';
}

// 获取文件列表
async function getFiles() {
    const result = await apiCall('/api/files');
    return result.data || [];
}

// 删除文件
async function deleteFile(shortId) {
    if (!confirm('确定要删除这个文件吗？')) return;
    
    const result = await apiCall(`/api/files/${shortId}`, {
        method: 'DELETE',
    });
    
    if (result.code === 'success') {
        loadFiles();
    } else {
        alert(result.message || '删除失败');
    }
}

// 加载文件列表
async function loadFiles() {
    const fileList = document.getElementById('file-list');
    if (!fileList) return;
    
    try {
        const files = await getFiles();
        
        if (files.length === 0) {
            fileList.innerHTML = '<li class="text-center" style="padding: 20px; color: #999;">暂无文件</li>';
            return;
        }
        
        fileList.innerHTML = files.map(file => `
            <li class="file-item">
                <div class="file-info">
                    <div class="file-name">${escapeHtml(file.filename)}</div>
                    <div class="file-meta">
                        ${formatSize(file.filesize)} | ${formatDate(file.upload_date)}
                        <a href="/d/${file.short_id}" target="_blank">查看</a>
                    </div>
                </div>
                <button class="btn btn-danger" onclick="deleteFile('${file.short_id}')">删除</button>
            </li>
        `).join('');
    } catch (error) {
        console.error('加载文件列表失败:', error);
    }
}

// 获取配置
async function getConfig() {
    const result = await apiCall('/api/app-config');
    return result.data || {};
}

// 保存配置
async function saveConfig(config) {
    const result = await apiCall('/api/app-config', {
        method: 'POST',
        body: JSON.stringify(config),
    });
    
    if (result.code === 'success') {
        alert('保存成功');
    } else {
        alert(result.message || '保存失败');
    }
}

// 设置密码
async function setPassword(password) {
    const result = await apiCall('/api/set-password', {
        method: 'POST',
        body: JSON.stringify({ password }),
    });
    
    if (result.code === 'success') {
        window.location.href = '/';
    } else {
        alert(result.message || '设置失败');
    }
}

// 工具函数
function escapeHtml(text) {
    const div = document.createElement('div');
    div.textContent = text;
    return div.innerHTML;
}

function formatSize(bytes) {
    if (bytes < 1024) return bytes + ' B';
    if (bytes < 1024 * 1024) return (bytes / 1024).toFixed(1) + ' KB';
    if (bytes < 1024 * 1024 * 1024) return (bytes / 1024 / 1024).toFixed(1) + ' MB';
    return (bytes / 1024 / 1024 / 1024).toFixed(1) + ' GB';
}

function formatDate(dateString) {
    const date = new Date(dateString);
    return date.toLocaleString('zh-CN');
}

// 页面初始化
document.addEventListener('DOMContentLoaded', function() {
    // 如果有文件列表，加载文件
    if (document.getElementById('file-list')) {
        loadFiles();
    }
});
