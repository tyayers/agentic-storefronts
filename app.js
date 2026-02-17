document.addEventListener("DOMContentLoaded", () => {
  // --- Configuration ---
  const routes = [
    {
      id: "storefronts",
      label: "Storefronts",
      icon: "store",
      file: "views/storefronts.html",
    },
    {
      id: "audiences",
      label: "Audiences",
      icon: "people",
      file: "views/audiences.html",
    },
    {
      id: "dashboard",
      label: "Dashboard",
      icon: "dashboard",
      file: "views/dashboard.html",
    },
    {
      id: "analytics",
      label: "Analytics",
      icon: "analytics",
      file: "views/analytics.html",
    },
    {
      id: "projects",
      label: "Projects",
      icon: "folder",
      file: "views/projects.html",
    },
    { id: "team", label: "Team", icon: "group", file: "views/team.html" },
    {
      id: "settings",
      label: "Settings",
      icon: "settings",
      file: "views/settings.html",
    },
  ];

  // --- State ---
  const state = {
    currentRoute: "storefronts",
    theme: localStorage.getItem("theme") || "light",
  };

  // --- Elements ---
  const navMenu = document.getElementById("nav-menu");
  const contentArea = document.getElementById("content-area");
  const pageTitle = document.getElementById("page-title");
  const themeToggleBtn = document.getElementById("theme-toggle");
  const themeIcon = themeToggleBtn.querySelector("span");

  const sidebarToggle = document.getElementById("sidebar-toggle");
  const sidebar = document.querySelector(".sidebar");
  const sidebarOverlay = document.getElementById("sidebar-overlay");

  // --- Initialization ---
  function init() {
    // Auth Check
    checkAuth();

    applyTheme(state.theme);

    // Handle initial route or hash change
    window.addEventListener("hashchange", handleRoute);

    // If no hash, default to dashboard and only if logged in (logic handled in checkAuth)
  }

  // --- Sidebar Logic ---
  function toggleSidebar() {
    const isOpen = sidebar.classList.contains("open");
    if (isOpen) {
      closeSidebar();
    } else {
      sidebar.classList.add("open");
      sidebarOverlay.classList.add("visible");
    }
  }

  function closeSidebar() {
    sidebar.classList.remove("open");
    sidebarOverlay.classList.remove("visible");
  }

  // --- Auth Logic ---
  function checkAuth() {
    const user = localStorage.getItem("user_session");
    const landingPage = document.getElementById("landing-page");
    const appContainer = document.getElementById("app-container");

    if (user) {
      // Logged In
      state.user = JSON.parse(user);
      landingPage.style.display = "none";
      appContainer.style.display = "flex"; // Restore flex layout

      updateUserProfile(state.user);
      renderNavigation();

      // If no hash, default to dashboard
      if (!window.location.hash) {
        window.location.hash = "storefronts";
      } else {
        handleRoute();
      }
    } else {
      // Logged Out
      landingPage.style.display = "flex"; // Show landing page
      appContainer.style.display = "none";
      window.location.hash = ""; // Clear hash
    }
  }

  // Exposed global function for Google Identity Services callback
  window.handleCredentialResponse = (response) => {
    // Decode JWT to get user info (Basic decoding)
    const responsePayload = decodeJwt(response.credential);

    const user = {
      name: responsePayload.name,
      email: responsePayload.email,
      picture: responsePayload.picture,
    };

    localStorage.setItem("user_session", JSON.stringify(user));
    checkAuth();
  };

  function decodeJwt(token) {
    var base64Url = token.split(".")[1];
    var base64 = base64Url.replace(/-/g, "+").replace(/_/g, "/");
    var jsonPayload = decodeURIComponent(
      window
        .atob(base64)
        .split("")
        .map(function (c) {
          return "%" + ("00" + c.charCodeAt(0).toString(16)).slice(-2);
        })
        .join(""),
    );

    return JSON.parse(jsonPayload);
  }

  function updateUserProfile(user) {
    const profileContainer = document.querySelector(".user-profile");
    if (profileContainer) {
      profileContainer.innerHTML = `
            <img src="${user.picture}" alt="User" class="avatar">
            <div class="user-info">
                <span class="user-name">${user.name}</span>
                <span class="user-role" style="font-size: 0.75rem; color: var(--text-secondary); cursor: pointer; text-decoration: underline;" id="btn-logout">Sign Out</span>
            </div>
        `;

      document.getElementById("btn-logout").addEventListener("click", () => {
        localStorage.removeItem("user_session");
        checkAuth();
      });
    }
  }

  // --- Navigation Logic ---
  function renderNavigation() {
    navMenu.innerHTML = routes
      .map(
        (route) => `
            <a href="#${route.id}" class="nav-item" data-id="${route.id}">
                <span class="material-icons-round">${route.icon}</span>
                <span>${route.label}</span>
            </a>
        `,
      )
      .join("");

    // Add click listeners to close sidebar on mobile selection
    navMenu.querySelectorAll(".nav-item").forEach((item) => {
      item.addEventListener("click", closeSidebar);
    });
  }

  function updateActiveNavLink(routeId) {
    document.querySelectorAll(".nav-item").forEach((item) => {
      if (item.dataset.id === routeId) {
        item.classList.add("active");
      } else {
        item.classList.remove("active");
      }
    });
  }

  async function handleRoute() {
    const hash = window.location.hash.substring(1) || "dashboard";
    const route = routes.find((r) => r.id === hash);

    if (route) {
      state.currentRoute = route.id;
      updateActiveNavLink(route.id);
      pageTitle.textContent = route.label;
      await loadContent(route.file);
    } else {
      contentArea.innerHTML = `
                <div class="fade-in">
                    <h2>404 - Page Not Found</h2>
                    <p>The requested page could not be found.</p>
                </div>
            `;
    }
  }

  async function loadContent(url) {
    // Show loader
    contentArea.innerHTML = '<div class="loader"></div>';

    try {
      // Simulate network delay for "app-like" feel
      // await new Promise(r => setTimeout(r, 300));

      const response = await fetch(url);
      if (!response.ok)
        throw new Error(`HTTP error! status: ${response.status}`);
      const html = await response.text();

      // Inject content
      contentArea.innerHTML = `<div class="fade-in">${html}</div>`;

      // Re-run any scripts in the injected content
      const scripts = contentArea.querySelectorAll("script");
      scripts.forEach((oldScript) => {
        const newScript = document.createElement("script");
        Array.from(oldScript.attributes).forEach((attr) =>
          newScript.setAttribute(attr.name, attr.value),
        );
        newScript.appendChild(document.createTextNode(oldScript.innerHTML));
        oldScript.parentNode.replaceChild(newScript, oldScript);
      });
    } catch (error) {
      console.error("Error loading content:", error);
      contentArea.innerHTML = `
                <div class="card fade-in" style="border-left: 4px solid #ef4444;">
                    <h3>Error Loading Content</h3>
                    <p>Could not load ${url}. Ensure the file exists.</p>
                </div>
            `;
    }
  }

  // --- Theme Logic ---
  function toggleTheme() {
    state.theme = state.theme === "light" ? "dark" : "light";
    localStorage.setItem("theme", state.theme);
    applyTheme(state.theme);
  }

  function applyTheme(theme) {
    document.documentElement.setAttribute("data-theme", theme);
    themeIcon.textContent = theme === "light" ? "light_mode" : "dark_mode";
  }

  // --- Event Listeners ---
  themeToggleBtn.addEventListener("click", toggleTheme);

  if (sidebarToggle) {
    sidebarToggle.addEventListener("click", toggleSidebar);
  }

  if (sidebarOverlay) {
    sidebarOverlay.addEventListener("click", closeSidebar);
  }

  // Start App
  init();
});
