/**
 * AI Marketplace - Main Application
 */
const App = {
  // Configuration
  storefrontId: "test-store-glha",
  demoMode: "true",
  apiHost: "",

  // State
  storefront: null,
  products: [],
  currentUser: null,
  currentCategory: null,
  currentView: "grid",
  currentSort: { field: "displayName", dir: "asc" },
  activeTag: null,
  searchQuery: "",
  viewCache: {},

  // ==========================================
  // Initialization
  // ==========================================
  async init() {
    this.initTheme();
    await this.loadData();
    this.hideSplash();
    this.bindGlobalEvents();
    this.initScrollAnimations();
    this.animateHeroStats();
  },

  // ==========================================
  // Theme
  // ==========================================
  initTheme() {
    const saved = localStorage.getItem("theme");
    if (saved) {
      document.documentElement.setAttribute("data-theme", saved);
    } else if (window.matchMedia("(prefers-color-scheme: dark)").matches) {
      document.documentElement.setAttribute("data-theme", "dark");
    }
    this.updateThemeIcon();
  },

  toggleTheme() {
    const current = document.documentElement.getAttribute("data-theme");
    const next = current === "dark" ? "light" : "dark";
    document.documentElement.setAttribute("data-theme", next);
    localStorage.setItem("theme", next);
    this.updateThemeIcon();
  },

  updateThemeIcon() {
    const icon = document.getElementById("theme-icon");
    if (!icon) return;
    const theme = document.documentElement.getAttribute("data-theme");
    icon.textContent = theme === "dark" ? "light_mode" : "dark_mode";
  },

  // ==========================================
  // Data Loading
  // ==========================================
  async loadData() {
    try {
      if (this.demoMode === "true") {
        const [sfRes, prodRes] = await Promise.all([
          fetch("mock-storefront.json"),
          fetch("mock-products.json"),
        ]);
        this.storefront = await sfRes.json();
        this.products = await prodRes.json();
      } else {
        const base = this.apiHost.replace(/\/$/, "");
        const [sfRes, prodRes] = await Promise.all([
          fetch(`${base}/api/storefronts/${this.storefrontId}`),
          fetch(`${base}/api/storefronts/${this.storefrontId}/products`),
        ]);
        this.storefront = await sfRes.json();
        this.products = await prodRes.json();
      }
      this.applyBranding();
    } catch (e) {
      console.error("Failed to load data:", e);
    }
  },

  applyBranding() {
    const name = this.storefront?.name || "AI Marketplace";
    document.title = name;
    document.querySelectorAll("#landing-brand-name, #app-brand-name, #footer-brand-name").forEach(
      (el) => (el.textContent = name)
    );
    const splashText = document.querySelector(".splash-text");
    if (splashText) splashText.textContent = name;
  },

  // ==========================================
  // Splash Screen
  // ==========================================
  hideSplash() {
    const splash = document.getElementById("splash-screen");
    setTimeout(() => {
      splash.classList.add("fade-out");
      setTimeout(() => {
        splash.classList.add("hidden");
        this.showLanding();
      }, 500);
    }, 800);
  },

  // ==========================================
  // Landing Page
  // ==========================================
  showLanding() {
    document.getElementById("landing-page").classList.remove("hidden");
    document.getElementById("app-shell").classList.add("hidden");
  },

  // ==========================================
  // Global Events
  // ==========================================
  bindGlobalEvents() {
    // Sign in buttons
    document.querySelectorAll("#landing-sign-in-btn, #hero-sign-in-btn, #cta-sign-in-btn").forEach(
      (btn) => btn.addEventListener("click", () => this.showSignIn())
    );

    // Sign in dialog
    document.getElementById("sign-in-close").addEventListener("click", () => this.hideSignIn());
    document.getElementById("sign-in-overlay").addEventListener("click", (e) => {
      if (e.target === e.currentTarget) this.hideSignIn();
    });

    // Demo sign-in
    document.getElementById("demo-sign-in-btn").addEventListener("click", () => this.demoSignIn());
    document.getElementById("demo-email").addEventListener("keydown", (e) => {
      if (e.key === "Enter") this.demoSignIn();
    });

    // Theme toggle
    document.getElementById("theme-toggle").addEventListener("click", () => this.toggleTheme());

    // User menu
    document.getElementById("user-avatar-btn").addEventListener("click", () => this.toggleUserMenu());
    document.addEventListener("click", (e) => {
      if (!e.target.closest("#user-menu")) {
        document.getElementById("user-dropdown").classList.add("hidden");
      }
    });

    // Sign out
    document.getElementById("sign-out-btn").addEventListener("click", () => this.signOut());
  },

  // ==========================================
  // Scroll Animations
  // ==========================================
  initScrollAnimations() {
    const observer = new IntersectionObserver(
      (entries) => {
        entries.forEach((entry) => {
          if (entry.isIntersecting) {
            entry.target.classList.add("visible");
          }
        });
      },
      { threshold: 0.1, rootMargin: "0px 0px -60px 0px" }
    );
    document.querySelectorAll(".animate-on-scroll").forEach((el) => observer.observe(el));
  },

  // ==========================================
  // Hero Stats Counter
  // ==========================================
  animateHeroStats() {
    document.querySelectorAll(".hero-stat-number").forEach((el) => {
      const target = parseInt(el.dataset.count);
      let current = 0;
      const step = target / 40;
      const timer = setInterval(() => {
        current += step;
        if (current >= target) {
          current = target;
          clearInterval(timer);
        }
        el.textContent = Math.floor(current);
      }, 30);
    });
  },

  // ==========================================
  // Authentication
  // ==========================================
  showSignIn() {
    const overlay = document.getElementById("sign-in-overlay");
    overlay.classList.remove("hidden");

    const authType = this.storefront?.authType || "demo";
    if (authType === "demo") {
      document.getElementById("demo-auth-form").classList.remove("hidden");
      document.getElementById("firebase-auth-form").classList.add("hidden");
    } else if (authType === "identity-platform") {
      document.getElementById("demo-auth-form").classList.add("hidden");
      document.getElementById("firebase-auth-form").classList.remove("hidden");
      this.initFirebaseAuth();
    }
  },

  hideSignIn() {
    document.getElementById("sign-in-overlay").classList.add("hidden");
  },

  demoSignIn() {
    const email = document.getElementById("demo-email").value || "user@example.com";
    this.currentUser = {
      email,
      displayName: email.split("@")[0],
      photoURL: null,
    };
    this.hideSignIn();
    this.enterApp();
  },

  async initFirebaseAuth() {
    // Dynamically load Firebase if not already loaded
    if (!window.firebase) {
      await this.loadScript("https://www.gstatic.com/firebasejs/9.23.0/firebase-app-compat.js");
      await this.loadScript("https://www.gstatic.com/firebasejs/9.23.0/firebase-auth-compat.js");
      await this.loadScript("https://www.gstatic.com/firebasejs/ui/6.1.0/firebase-ui-auth.js");

      const link = document.createElement("link");
      link.rel = "stylesheet";
      link.href = "https://www.gstatic.com/firebasejs/ui/6.1.0/firebase-ui-auth.css";
      document.head.appendChild(link);
    }

    if (!firebase.apps.length) {
      firebase.initializeApp({
        apiKey: this.storefront.authApiKey,
        authDomain: this.storefront.authDomain,
      });
    }

    const uiConfig = {
      signInSuccessUrl: window.location.href,
      signInOptions: [
        firebase.auth.GoogleAuthProvider.PROVIDER_ID,
        firebase.auth.EmailAuthProvider.PROVIDER_ID,
        {
          provider: "saml.enterprise",
          providerName: "Enterprise SSO",
        },
      ],
      callbacks: {
        signInSuccessWithAuthResult: (authResult) => {
          const user = authResult.user;
          this.currentUser = {
            email: user.email,
            displayName: user.displayName || user.email.split("@")[0],
            photoURL: user.photoURL,
          };
          this.hideSignIn();
          this.enterApp();
          return false;
        },
      },
    };

    const ui =
      firebaseui.auth.AuthUI.getInstance() || new firebaseui.auth.AuthUI(firebase.auth());
    ui.start("#firebase-auth-container", uiConfig);
  },

  loadScript(src) {
    return new Promise((resolve, reject) => {
      const script = document.createElement("script");
      script.src = src;
      script.onload = resolve;
      script.onerror = reject;
      document.head.appendChild(script);
    });
  },

  signOut() {
    this.currentUser = null;
    document.getElementById("user-dropdown").classList.add("hidden");
    document.getElementById("app-shell").classList.add("hidden");
    this.showLanding();
    this.showToast("Signed out successfully", "logout");
  },

  // ==========================================
  // App Shell
  // ==========================================
  enterApp() {
    document.getElementById("landing-page").classList.add("hidden");
    document.getElementById("app-shell").classList.remove("hidden");
    document.getElementById("app-shell").classList.add("page-enter");

    this.updateUserUI();
    this.renderCategoryNav();

    // Open first category
    const categories = this.getCategories();
    if (categories.length > 0) {
      this.openCategory(categories[0]);
    }
  },

  updateUserUI() {
    const user = this.currentUser;
    if (!user) return;

    const initials = user.displayName
      ? user.displayName
          .split(" ")
          .map((w) => w[0])
          .join("")
          .toUpperCase()
          .slice(0, 2)
      : "U";

    const avatarEl = document.getElementById("user-avatar");
    const dropdownAvatarEl = document.getElementById("dropdown-avatar");

    if (user.photoURL) {
      avatarEl.innerHTML = `<img src="${this.escapeHtml(user.photoURL)}" alt="Avatar">`;
      dropdownAvatarEl.innerHTML = `<img src="${this.escapeHtml(user.photoURL)}" alt="Avatar">`;
    } else {
      avatarEl.textContent = initials;
      dropdownAvatarEl.textContent = initials;
    }

    document.getElementById("dropdown-user-name").textContent = user.displayName || "User";
    document.getElementById("dropdown-user-email").textContent = user.email;
  },

  toggleUserMenu() {
    const dropdown = document.getElementById("user-dropdown");
    dropdown.classList.toggle("hidden");
  },

  getCategories() {
    const cats = new Set();
    this.products.forEach((p) => {
      (p.categories || []).forEach((c) => cats.add(c));
    });
    return Array.from(cats).sort();
  },

  renderCategoryNav() {
    const nav = document.getElementById("category-nav");
    const categories = this.getCategories();
    nav.innerHTML = categories
      .map(
        (cat) =>
          `<button class="category-link" data-category="${this.escapeHtml(cat)}">${this.escapeHtml(cat)}</button>`
      )
      .join("");

    nav.querySelectorAll(".category-link").forEach((btn) => {
      btn.addEventListener("click", () => {
        this.openCategory(btn.dataset.category);
      });
    });
  },

  // ==========================================
  // View Loading
  // ==========================================
  async loadView(viewName) {
    if (this.viewCache[viewName]) return this.viewCache[viewName];

    const res = await fetch(`views/${viewName}.html`);
    const html = await res.text();
    this.viewCache[viewName] = html;
    return html;
  },

  async openCategory(category) {
    this.currentCategory = category;
    this.searchQuery = "";
    this.activeTag = null;

    // Update nav active state
    document.querySelectorAll(".category-link").forEach((btn) => {
      btn.classList.toggle("active", btn.dataset.category === category);
    });

    const main = document.getElementById("app-main");
    const html = await this.loadView("catalog");
    main.innerHTML = html;
    main.classList.add("page-enter");

    this.initCatalogView(category);
  },

  async openProductDetail(productId) {
    const product = this.products.find((p) => p.id === productId);
    if (!product) return;

    const main = document.getElementById("app-main");
    const html = await this.loadView("product-detail");
    main.innerHTML = html;
    main.classList.add("page-enter");

    this.initProductDetailView(product);
  },

  // ==========================================
  // Catalog View
  // ==========================================
  initCatalogView(category) {
    const categoryProducts = this.products.filter((p) =>
      (p.categories || []).includes(category)
    );

    // Collect unique tags
    const tags = new Set();
    categoryProducts.forEach((p) => (p.tags || []).forEach((t) => tags.add(t)));

    // Render tags
    const tagsContainer = document.getElementById("catalog-tags");
    tagsContainer.innerHTML = Array.from(tags)
      .sort()
      .map(
        (tag) =>
          `<button class="tag-chip" data-tag="${this.escapeHtml(tag)}">${this.escapeHtml(tag)}</button>`
      )
      .join("");

    tagsContainer.querySelectorAll(".tag-chip").forEach((chip) => {
      chip.addEventListener("click", () => {
        this.activeTag = this.activeTag === chip.dataset.tag ? null : chip.dataset.tag;
        tagsContainer.querySelectorAll(".tag-chip").forEach((c) => {
          c.classList.toggle("active", c.dataset.tag === this.activeTag);
        });
        this.renderProducts(categoryProducts);
      });
    });

    // Search
    const searchInput = document.getElementById("catalog-search-input");
    searchInput.addEventListener("input", () => {
      this.searchQuery = searchInput.value.toLowerCase();
      this.renderProducts(categoryProducts);
    });

    // View toggle
    document.getElementById("grid-view-btn").addEventListener("click", () => {
      this.currentView = "grid";
      document.getElementById("grid-view-btn").classList.add("active");
      document.getElementById("list-view-btn").classList.remove("active");
      this.renderProducts(categoryProducts);
    });

    document.getElementById("list-view-btn").addEventListener("click", () => {
      this.currentView = "list";
      document.getElementById("list-view-btn").classList.add("active");
      document.getElementById("grid-view-btn").classList.remove("active");
      this.renderProducts(categoryProducts);
    });

    this.renderProducts(categoryProducts);
  },

  filterProducts(products) {
    return products.filter((p) => {
      const matchesSearch =
        !this.searchQuery ||
        p.displayName.toLowerCase().includes(this.searchQuery) ||
        (p.tags || []).some((t) => t.toLowerCase().includes(this.searchQuery));
      const matchesTag = !this.activeTag || (p.tags || []).includes(this.activeTag);
      return matchesSearch && matchesTag;
    });
  },

  sortProducts(products) {
    const { field, dir } = this.currentSort;
    return [...products].sort((a, b) => {
      let va = a[field] || "";
      let vb = b[field] || "";
      if (typeof va === "string") va = va.toLowerCase();
      if (typeof vb === "string") vb = vb.toLowerCase();
      if (va < vb) return dir === "asc" ? -1 : 1;
      if (va > vb) return dir === "asc" ? 1 : -1;
      return 0;
    });
  },

  renderProducts(allProducts) {
    const container = document.getElementById("catalog-products-container");
    const filtered = this.sortProducts(this.filterProducts(allProducts));

    if (filtered.length === 0) {
      container.innerHTML = `
        <div class="empty-state">
          <span class="material-icons">search_off</span>
          <h3>No APIs found</h3>
          <p>Try adjusting your search or filters.</p>
        </div>`;
      return;
    }

    if (this.currentView === "grid") {
      container.innerHTML = `<div class="products-grid">${filtered.map((p) => this.renderProductCard(p)).join("")}</div>`;
    } else {
      container.innerHTML = this.renderProductList(filtered);
    }

    // Bind click events
    container.querySelectorAll("[data-product-id]").forEach((el) => {
      el.addEventListener("click", () => this.openProductDetail(el.dataset.productId));
    });
  },

  renderProductCard(product) {
    const styleClass = (product.displayStyle || "").toLowerCase();
    const icon = styleClass === "mcp" ? "settings_input_component" : "api";
    const endpoints = (product.endpoints || []).slice(0, 2);

    return `
      <div class="product-card" data-product-id="${this.escapeHtml(product.id)}">
        <div class="product-card-header">
          <div class="product-icon ${styleClass}">
            <span class="material-icons">${icon}</span>
          </div>
          <div class="product-card-info">
            <div class="product-card-name">${this.escapeHtml(product.displayName)}</div>
            <span class="product-card-style">${this.escapeHtml(product.displayStyle || "API")}</span>
          </div>
        </div>
        <div class="product-card-body">
          ${endpoints
            .map(
              (ep) =>
                `<div class="product-card-endpoint"><span class="material-icons">link</span>${this.escapeHtml(ep)}</div>`
            )
            .join("")}
          ${(product.endpoints || []).length > 2 ? `<div class="product-card-endpoints">+${product.endpoints.length - 2} more endpoints</div>` : ""}
        </div>
        <div class="product-card-tags">
          ${(product.tags || []).map((t) => `<span class="product-tag">${this.escapeHtml(t)}</span>`).join("")}
        </div>
      </div>`;
  },

  renderProductList(products) {
    const sortIcon = this.currentSort.dir === "asc" ? "arrow_upward" : "arrow_downward";
    const sortField = this.currentSort.field;

    const headerCell = (label, field) => {
      const sorted = sortField === field ? "sorted" : "";
      return `<div class="list-header-cell ${sorted}" data-sort-field="${field}">
        ${label}
        <span class="material-icons">${sortIcon}</span>
      </div>`;
    };

    let html = `
      <div class="products-list">
        <div class="products-list-header">
          ${headerCell("Name", "displayName")}
          ${headerCell("Style", "displayStyle")}
          ${headerCell("Category", "categories")}
          ${headerCell("Tags", "tags")}
        </div>`;

    products.forEach((p) => {
      const styleClass = (p.displayStyle || "").toLowerCase();
      const icon = styleClass === "mcp" ? "settings_input_component" : "api";
      html += `
        <div class="product-list-item" data-product-id="${this.escapeHtml(p.id)}">
          <div class="list-item-name">
            <div class="product-icon ${styleClass}"><span class="material-icons">${icon}</span></div>
            <span>${this.escapeHtml(p.displayName)}</span>
          </div>
          <div class="list-item-style">${this.escapeHtml(p.displayStyle || "API")}</div>
          <div class="list-item-category">${this.escapeHtml((p.categories || []).join(", "))}</div>
          <div class="list-item-tags">
            ${(p.tags || []).map((t) => `<span class="product-tag">${this.escapeHtml(t)}</span>`).join("")}
          </div>
        </div>`;
    });

    html += "</div>";

    // After rendering, we attach sort handlers
    setTimeout(() => {
      document.querySelectorAll(".list-header-cell").forEach((cell) => {
        cell.addEventListener("click", () => {
          const field = cell.dataset.sortField;
          if (this.currentSort.field === field) {
            this.currentSort.dir = this.currentSort.dir === "asc" ? "desc" : "asc";
          } else {
            this.currentSort = { field, dir: "asc" };
          }
          const categoryProducts = this.products.filter((p) =>
            (p.categories || []).includes(this.currentCategory)
          );
          this.renderProducts(categoryProducts);
        });
      });
    }, 0);

    return html;
  },

  // ==========================================
  // Product Detail View
  // ==========================================
  initProductDetailView(product) {
    const styleClass = (product.displayStyle || "").toLowerCase();
    const icon = styleClass === "mcp" ? "settings_input_component" : "api";

    // Icon
    const iconEl = document.getElementById("detail-icon");
    iconEl.classList.add(styleClass);
    iconEl.querySelector(".material-icons").textContent = icon;

    // Name
    document.getElementById("detail-name").textContent = product.displayName;

    // Style badge
    const styleBadge = document.getElementById("detail-style-badge");
    styleBadge.textContent = product.displayStyle || "API";
    styleBadge.classList.add(styleClass);

    // Category
    document.getElementById("detail-category-badge").innerHTML = `
      <span class="material-icons" style="font-size:16px">folder</span>
      ${this.escapeHtml((product.categories || []).join(", "))}
    `;

    // Tags
    const tagsEl = document.getElementById("detail-tags");
    tagsEl.innerHTML = (product.tags || [])
      .map((t) => `<span class="tag-chip">${this.escapeHtml(t)}</span>`)
      .join("");

    // Back button
    document.getElementById("detail-back-btn").addEventListener("click", () => {
      if (this.currentCategory) this.openCategory(this.currentCategory);
    });

    // Build tabs based on product style
    const isRest = styleClass === "rest";
    const hasSpec = !!product.specContents;

    const tabs = [];
    if (isRest && hasSpec) {
      tabs.push({ id: "api-reference", label: "API Reference", icon: "description" });
    }
    tabs.push({ id: "endpoints", label: "Endpoints", icon: "link" });
    tabs.push({ id: "code-snippets", label: "Code Snippets", icon: "code" });
    tabs.push({ id: "documentation", label: "Documentation", icon: "menu_book" });

    const tabsEl = document.getElementById("detail-tabs");
    tabsEl.innerHTML = tabs
      .map(
        (tab, i) =>
          `<button class="detail-tab ${i === 0 ? "active" : ""}" data-tab="${tab.id}">
            <span class="material-icons" style="font-size:16px;vertical-align:middle;margin-right:4px">${tab.icon}</span>
            ${tab.label}
          </button>`
      )
      .join("");

    tabsEl.querySelectorAll(".detail-tab").forEach((tabBtn) => {
      tabBtn.addEventListener("click", () => {
        tabsEl.querySelectorAll(".detail-tab").forEach((t) => t.classList.remove("active"));
        tabBtn.classList.add("active");
        this.renderDetailTab(tabBtn.dataset.tab, product);
      });
    });

    // Render first tab
    this.renderDetailTab(tabs[0].id, product);
  },

  renderDetailTab(tabId, product) {
    const container = document.getElementById("detail-tab-content");

    switch (tabId) {
      case "api-reference":
        this.renderApiReference(container, product);
        break;
      case "endpoints":
        this.renderEndpoints(container, product);
        break;
      case "code-snippets":
        this.renderCodeSnippets(container, product);
        break;
      case "documentation":
        this.renderDocumentation(container, product);
        break;
    }
  },

  renderApiReference(container, product) {
    // Decode base64 spec
    let specText = "";
    try {
      specText = atob(product.specContents);
    } catch (e) {
      specText = product.specContents;
    }

    const theme = document.documentElement.getAttribute("data-theme");

    // Create an iframe with Scalar
    const scalarHtml = `<!DOCTYPE html>
<html>
<head>
  <meta charset="utf-8">
  <style>
    body { margin: 0; font-family: sans-serif; }
  </style>
</head>
<body>
  <script id="api-reference" data-configuration='${JSON.stringify({ theme: theme === "dark" ? "dark" : "light" }).replace(/'/g, "&#39;")}'></script>
  <script>
    document.getElementById('api-reference').dataset.configuration = JSON.stringify({
      theme: '${theme === "dark" ? "dark" : "light"}',
      spec: { content: ${JSON.stringify(specText)} },
      hideDownloadButton: true
    });
  </script>
  <script src="https://cdn.jsdelivr.net/npm/@scalar/api-reference"></script>
</body>
</html>`;

    container.innerHTML = `<div class="scalar-container"><iframe id="scalar-iframe" sandbox="allow-scripts allow-same-origin"></iframe></div>`;

    const iframe = document.getElementById("scalar-iframe");
    iframe.srcdoc = scalarHtml;
  },

  renderEndpoints(container, product) {
    const endpoints = product.endpoints || [];
    if (endpoints.length === 0) {
      container.innerHTML = `<div class="empty-state"><span class="material-icons">link_off</span><h3>No endpoints</h3><p>No endpoints are configured for this product.</p></div>`;
      return;
    }

    container.innerHTML = `
      <div class="endpoints-list">
        ${endpoints
          .map(
            (ep) => `
          <div class="endpoint-item">
            <span class="endpoint-method">GET</span>
            <span class="endpoint-url">${this.escapeHtml(ep)}</span>
            <button class="btn btn-text btn-small endpoint-copy" data-url="${this.escapeHtml(ep)}">
              <span class="material-icons" style="font-size:16px">content_copy</span>
              Copy
            </button>
          </div>`
          )
          .join("")}
      </div>`;

    container.querySelectorAll(".endpoint-copy").forEach((btn) => {
      btn.addEventListener("click", (e) => {
        e.stopPropagation();
        navigator.clipboard.writeText(btn.dataset.url);
        this.showToast("URL copied to clipboard", "check_circle");
      });
    });
  },

  renderCodeSnippets(container, product) {
    const endpoint = (product.endpoints || [])[0] || "https://api.example.com/v1";
    const isMcp = (product.displayStyle || "").toLowerCase() === "mcp";

    if (isMcp) {
      container.innerHTML = `
        <div class="code-block">
          <div class="code-block-header">
            <span>MCP Client Configuration (JSON)</span>
            <button class="btn btn-text btn-small copy-code-btn">
              <span class="material-icons" style="font-size:16px">content_copy</span> Copy
            </button>
          </div>
          <pre><code>{
  "mcpServers": {
    "${this.escapeHtml(product.name || "server")}": {
      "url": "${this.escapeHtml(endpoint)}"
    }
  }
}</code></pre>
        </div>

        <div class="code-block">
          <div class="code-block-header">
            <span>Python - MCP Client</span>
            <button class="btn btn-text btn-small copy-code-btn">
              <span class="material-icons" style="font-size:16px">content_copy</span> Copy
            </button>
          </div>
          <pre><code>from mcp import ClientSession
from mcp.client.streamable_http import streamablehttp_client

async def main():
    async with streamablehttp_client("${this.escapeHtml(endpoint)}") as (read, write, _):
        async with ClientSession(read, write) as session:
            await session.initialize()

            # List available tools
            tools = await session.list_tools()
            print("Available tools:", tools)

            # Call a tool
            result = await session.call_tool("tool_name", {"param": "value"})
            print("Result:", result)

import asyncio
asyncio.run(main())</code></pre>
        </div>`;
    } else {
      container.innerHTML = `
        <div class="code-block">
          <div class="code-block-header">
            <span>cURL</span>
            <button class="btn btn-text btn-small copy-code-btn">
              <span class="material-icons" style="font-size:16px">content_copy</span> Copy
            </button>
          </div>
          <pre><code>curl -X GET "${this.escapeHtml(endpoint)}" \\
  -H "Authorization: Bearer YOUR_API_KEY" \\
  -H "Content-Type: application/json"</code></pre>
        </div>

        <div class="code-block">
          <div class="code-block-header">
            <span>Python</span>
            <button class="btn btn-text btn-small copy-code-btn">
              <span class="material-icons" style="font-size:16px">content_copy</span> Copy
            </button>
          </div>
          <pre><code>import requests

response = requests.get(
    "${this.escapeHtml(endpoint)}",
    headers={
        "Authorization": "Bearer YOUR_API_KEY",
        "Content-Type": "application/json"
    }
)

print(response.json())</code></pre>
        </div>

        <div class="code-block">
          <div class="code-block-header">
            <span>JavaScript (fetch)</span>
            <button class="btn btn-text btn-small copy-code-btn">
              <span class="material-icons" style="font-size:16px">content_copy</span> Copy
            </button>
          </div>
          <pre><code>const response = await fetch("${this.escapeHtml(endpoint)}", {
  headers: {
    "Authorization": "Bearer YOUR_API_KEY",
    "Content-Type": "application/json"
  }
});

const data = await response.json();
console.log(data);</code></pre>
        </div>`;
    }

    container.querySelectorAll(".copy-code-btn").forEach((btn) => {
      btn.addEventListener("click", () => {
        const code = btn.closest(".code-block").querySelector("code").textContent;
        navigator.clipboard.writeText(code);
        this.showToast("Code copied to clipboard", "check_circle");
      });
    });
  },

  renderDocumentation(container, product) {
    const isMcp = (product.displayStyle || "").toLowerCase() === "mcp";
    const endpoints = product.endpoints || [];

    container.innerHTML = `
      <div style="max-width:720px">
        <h2 style="margin-bottom:16px">${this.escapeHtml(product.displayName)}</h2>

        <div style="margin-bottom:24px; padding:20px; background:var(--md-surface-container-low); border-radius:12px; border:1px solid var(--md-outline-variant)">
          <h3 style="font-size:16px; margin-bottom:12px; display:flex; align-items:center; gap:8px">
            <span class="material-icons" style="font-size:20px; color:var(--md-primary)">info</span>
            Overview
          </h3>
          <p style="font-size:14px; color:var(--md-on-surface-variant); line-height:1.7">
            ${this.escapeHtml(product.displayName)} is a ${isMcp ? "Model Context Protocol (MCP) server" : "RESTful API"}
            available in the <strong>${this.escapeHtml((product.categories || []).join(", "))}</strong> category.
            ${isMcp ? "It can be used with any MCP-compatible client, enabling seamless integration with AI agents and tools." : "It provides standard HTTP endpoints for integration into your applications."}
          </p>
        </div>

        <div style="margin-bottom:24px; padding:20px; background:var(--md-surface-container-low); border-radius:12px; border:1px solid var(--md-outline-variant)">
          <h3 style="font-size:16px; margin-bottom:12px; display:flex; align-items:center; gap:8px">
            <span class="material-icons" style="font-size:20px; color:var(--md-primary)">link</span>
            ${isMcp ? "Server URL" : "Base URLs"}
          </h3>
          ${endpoints
            .map(
              (ep) =>
                `<div style="padding:8px 12px; background:var(--md-surface-container); border-radius:8px; margin-bottom:8px; font-family:monospace; font-size:13px; color:var(--md-primary); word-break:break-all">${this.escapeHtml(ep)}</div>`
            )
            .join("")}
        </div>

        <div style="margin-bottom:24px; padding:20px; background:var(--md-surface-container-low); border-radius:12px; border:1px solid var(--md-outline-variant)">
          <h3 style="font-size:16px; margin-bottom:12px; display:flex; align-items:center; gap:8px">
            <span class="material-icons" style="font-size:20px; color:var(--md-primary)">rocket_launch</span>
            Getting Started
          </h3>
          <ol style="font-size:14px; color:var(--md-on-surface-variant); line-height:2; padding-left:20px">
            <li>Obtain your API key from the developer portal</li>
            ${isMcp ? "<li>Configure your MCP client with the server URL above</li>" : "<li>Set up your HTTP client with the base URL above</li>"}
            <li>Include your API key in the Authorization header</li>
            <li>Start making ${isMcp ? "tool calls" : "API requests"}!</li>
          </ol>
        </div>

        <div style="padding:20px; background:var(--md-surface-container-low); border-radius:12px; border:1px solid var(--md-outline-variant)">
          <h3 style="font-size:16px; margin-bottom:12px; display:flex; align-items:center; gap:8px">
            <span class="material-icons" style="font-size:20px; color:var(--md-primary)">security</span>
            Authentication
          </h3>
          <p style="font-size:14px; color:var(--md-on-surface-variant); line-height:1.7">
            All requests must include a valid API key. Pass your key using the <code style="padding:2px 6px; background:var(--md-surface-container); border-radius:4px; font-size:12px">Authorization: Bearer YOUR_API_KEY</code> header.
          </p>
        </div>
      </div>`;
  },

  // ==========================================
  // Utilities
  // ==========================================
  escapeHtml(str) {
    if (!str) return "";
    const div = document.createElement("div");
    div.textContent = String(str);
    return div.innerHTML;
  },

  showToast(message, icon = "info") {
    let toast = document.querySelector(".toast");
    if (!toast) {
      toast = document.createElement("div");
      toast.className = "toast";
      document.body.appendChild(toast);
    }
    toast.innerHTML = `<span class="material-icons">${icon}</span>${this.escapeHtml(message)}`;
    toast.classList.remove("show");
    // Force reflow
    void toast.offsetWidth;
    toast.classList.add("show");
    setTimeout(() => toast.classList.remove("show"), 3000);
  },
};

// Start the app
document.addEventListener("DOMContentLoaded", () => App.init());
