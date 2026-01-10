// loader.js - Client-side component loader

async function loadComponent(elementId, componentPath) {
    try {
        const response = await fetch(componentPath);
        if (!response.ok) {
            throw new Error(`Failed to load ${componentPath}: ${response.status} ${response.statusText}`);
        }
        const html = await response.text();
        const container = document.getElementById(elementId);
        if (container) {
            container.innerHTML = html;
        } else {
            console.error(`Container #${elementId} not found for component ${componentPath}`);
        }
    } catch (error) {
        console.error(`Error loading component ${componentPath}:`, error);
        // Optional: show a user-visible error
    }
}

async function initApplication() {
    // Check authentication first
    try {
        const authResp = await fetch('/api/auth/me');
        if (!authResp.ok) {
            window.location.href = '/login.html';
            return;
        }
        window.currentUser = await authResp.json();
    } catch (e) {
        window.location.href = '/login.html';
        return;
    }

    // Load all components in parallel
    await Promise.all([
        loadComponent('mobile-header', 'components/header.html'),
        loadComponent('left-sidebar', 'components/sidebar.html'),
        loadComponent('main-area', 'components/main-area.html'),
        loadComponent('right-sidebar', 'components/settings.html'),
        loadComponent('modals-container', 'components/modals.html'),
        loadComponent('overlays-container', 'components/overlays.html')
    ]);

    // Initialize the main application logic
    if (window.initializeApp) {
        // Allow DOM to update before running init
        setTimeout(() => {
            window.initializeApp();
        }, 50);
    } else {
        console.error("window.initializeApp not found. Main logic not started.");
    }
}

// Start loading when the DOM is ready
document.addEventListener('DOMContentLoaded', initApplication);