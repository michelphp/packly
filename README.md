
# 📦 Packly

**Packly** is an ultra-lightweight, zero-dependency asset builder and frontend package manager written in Go. It allows you to manage dependencies (Bootstrap, FontAwesome, Leaflet, etc.) and compile Sass/SCSS files for your web applications **without Node.js, npm, or Webpack/Vite**.

Packly is built on a strong philosophy: **No hidden magic.** It downloads what you ask for and compiles your Sass. It does not parse your AST, it does not tree-shake your JavaScript, and it will never auto-inject tags into your HTML. You are the developer, you stay in control.

---

## ⚡ Why Packly? (The "Plus")

### 🚫 Zero Node.js / npm

No `node_modules` weighing 500MB, no package lock files, no JavaScript tooling complexity. Packly is a single, fast Go binary.

### 🌐 Direct CDN & ZIP Downloads

Declare URLs from jsDelivr, unpkg, or direct GitHub releases, and Packly vendors them locally into your project. Need a minified file? Point to the `.min.js`. Need an ES Module? Point to the `+esm` build.

### 🎨 Built-in Auto-Sass

Just provide the version number. Packly automatically detects your OS and Architecture (Windows, Mac ARM, Linux x64, etc.), downloads the correct `dart-sass` binary, and compiles your stylesheets seamlessly.

### 🧙‍♂️ No Hidden Magic

Tired of tools generating `<script type="importmap">` behind your back? Packly respects your intelligence. It prepares your files in the `public/` directory, and leaves the HTML integration entirely up to you. You write your own `<script>` tags, your own import maps, and you keep total control over your frontend architecture.

---

## 🆚 Comparison

| Feature | npm / Vite / Webpack | Symfony AssetMapper | **Packly** |
| --- | --- | --- | --- |
| **Runtime** | Heavy Node.js required | PHP (Symfony-tied) | **Zero-dependency Go Binary** |
| **Setup Time** | High (config files, scripts) | Low (built-in Symfony) | **Instant (1 small `.ini` file)** |
| **Sass Support** | Needs loader/plugin configuration | Requires manual CLI setup | **Automated (download & compile)** |
| **Philosophy** | "We bundle everything" | "We hide the import maps" | **"Explicit: You write your HTML tags"** |

---

## 🚀 Quick Start (KISS)

### 1. Configure (`packly.ini`)

Create a `packly.ini` at the root of your project. This acts as both your configuration and your dependency lock:

```ini
[build]
# Directory where your source assets (Sass, images, custom JS) are located
source = assets
# Target directory where the built assets will be published (copied to <target>/build/)
target = public
# Comma-separated list of paths (relative to 'source') to exclude from the build target
# Note: Packly automatically excludes .scss/.sass source files from being copied to public/
ignore = vendor/bootstrap

[dependencies]
# PRINCIPLE: 
# KEY   = The name of the directory created under `<source>/vendor/<KEY>/`
# VALUE = Comma-separated list of URLs to download.
#         - If ZIP file: downloaded and automatically unzipped inside `<source>/vendor/<KEY>/`
#         - If single file: downloaded directly as `<source>/vendor/<KEY>/<filename>`
jquery = https://cdn.jsdelivr.net/npm/jquery@3.7.1/dist/jquery.min.js
leaflet = https://unpkg.com/leaflet@1.9.4/dist/leaflet.js, https://unpkg.com/leaflet/dist/leaflet.css
bootstrap = https://github.com/twbs/bootstrap/archive/refs/tags/v5.3.3.zip

[sass]
# Version of dart-sass used for compilation (Packly fetches the right binary for your OS)
version = 1.79.2
# Sass entrypoint files (relative to 'source') compiled to CSS during build
entrypoints = styles/app.scss, styles/admin.scss

```

### 2. Run Commands

* **Install dependencies** (Downloads/unzips assets under `assets/vendor/<key>/`):
```bash
packly install
```


*(Add `--force` to reinstall all dependencies)*
* **Build & Compile** (Copies raw assets to `public/build/` and compiles Sass entrypoints):
```bash
packly build
```


* **Watch Mode** (Watch for changes in the `assets/` directory and compile on the fly):
```bash
packly build --watch
```

---

## 📖 Concrete Example: Modern ES6 Modules Workflow

To help you visualize how Packly interacts with native browser standards, here is a complete, end-to-end example using a modern ES6 module. We will use `canvas-confetti` as it is visual and perfect for demonstrating native ES modules.

### 1. Configuration (`packly.ini`)

Define your dependencies by pointing directly to an ES Module (`.mjs`) build on a CDN.

```ini
[build]
source = assets
target = public

[dependencies]
# We fetch the ES module version directly
confetti = https://cdn.jsdelivr.net/npm/canvas-confetti@1.9.2/dist/confetti.module.mjs

[sass]
version = 1.79.2
entrypoints = styles/app.scss

```

### 2. Directory Workflow (Before / After)

Here is exactly how Packly processes and maps your physical files:

**Phase A: Running `packly install**`
Packly reads your `.ini` file and downloads the module locally into your source directory:

* `assets/vendor/confetti/confetti.module.mjs`

**Phase B: Your Custom Assets**
You create your own custom JavaScript entrypoint file in your source folder:

* `assets/js/app.js`

**Phase C: Running `packly build**`
Packly performs a clean, straightforward mirror copy of your `assets/` folder over to `public/build/` (while intelligently omitting raw `.scss` files). Your final production-ready directory structure exposed to the web becomes:

* `public/build/vendor/confetti/confetti.module.mjs`
* `public/build/js/app.js`
* `public/build/styles/app.css` *(compiled automatically by the Sass engine)*

---

### 3. Your JavaScript Code (`assets/js/app.js`)

You write standard, modern JavaScript using **bare module specifiers**. You don't need to specify complex, relative physical paths in your application code; the native browser handles that using your import map.

```javascript
// assets/js/app.js
import confetti from 'confetti';

document.addEventListener('DOMContentLoaded', () => {
    const btn = document.getElementById('btn-party');
    
    if (btn) {
        btn.addEventListener('click', () => {
            confetti({
                particleCount: 100,
                spread: 70,
                origin: { y: 0.6 }
            });
            console.log("Native ES6 Modules running successfully!");
        });
    }
});

```

---

### 4. Native HTML Integration (`index.html`)

This is where you assume full responsibility and control over your frontend. Packly handles the backend mechanics (downloading and putting assets in the right directory), and leaves the structural `<script>` and `<link>` declarations explicitly up to you.

```html
<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <title>Packly ES6 Native Example</title>
    
    <link rel="stylesheet" href="/build/styles/app.css">

    <script type="importmap">
    {
      "imports": {
        "confetti": "/build/vendor/confetti/confetti.module.mjs"
      }
    }
    </script>
</head>
<body>

    <button id="btn-party">Throw Confetti!</button>

    <script type="module" src="/build/js/app.js"></script>

</body>
</html>

```

# LICENCE

See [LICENCE](LICENCE) file