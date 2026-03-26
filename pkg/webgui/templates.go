package webgui

const indexHTML = `<!DOCTYPE html>
<html lang="ja">
<head>
<meta charset="UTF-8">
<meta name="viewport" content="width=device-width, initial-scale=1.0">
<title>川柳一覧 - FindSenryu</title>
<style>
* { margin: 0; padding: 0; box-sizing: border-box; }
body {
  font-family: 'Noto Sans JP', 'Hiragino Kaku Gothic ProN', sans-serif;
  background: #f5f0e8;
  color: #333;
  min-height: 100vh;
}
.header {
  background: linear-gradient(135deg, #8b4513, #a0522d);
  color: white;
  padding: 24px;
  text-align: center;
}
.header h1 { font-size: 1.8em; font-weight: 700; }
.header p { margin-top: 8px; opacity: 0.8; }
.container {
  max-width: 1000px;
  margin: 0 auto;
  padding: 24px;
}
.filter-bar {
  display: flex;
  gap: 12px;
  margin-bottom: 24px;
  flex-wrap: wrap;
}
.filter-bar input, .filter-bar button {
  padding: 10px 16px;
  border: 1px solid #ccc;
  border-radius: 6px;
  font-size: 14px;
}
.filter-bar button {
  background: #8b4513;
  color: white;
  border: none;
  cursor: pointer;
}
.filter-bar button:hover { background: #a0522d; }
.senryu-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(280px, 1fr));
  gap: 20px;
}
.senryu-card {
  background: white;
  border-radius: 12px;
  overflow: hidden;
  box-shadow: 0 2px 8px rgba(0,0,0,0.1);
  transition: transform 0.2s;
}
.senryu-card:hover { transform: translateY(-4px); }
.senryu-card img {
  width: 100%;
  object-fit: contain;
  background: #faf8f5;
}
.senryu-card .info {
  padding: 12px 16px;
  border-top: 1px solid #eee;
}
.senryu-card .text {
  font-size: 14px;
  writing-mode: horizontal-tb;
  margin-bottom: 6px;
}
.senryu-card .meta {
  font-size: 12px;
  color: #888;
}
.pagination {
  display: flex;
  justify-content: center;
  gap: 8px;
  margin-top: 32px;
}
.pagination button {
  padding: 8px 16px;
  border: 1px solid #ccc;
  border-radius: 6px;
  background: white;
  cursor: pointer;
}
.pagination button:hover { background: #f0ebe3; }
.pagination button.active {
  background: #8b4513;
  color: white;
  border-color: #8b4513;
}
.spoiler-blur { filter: blur(8px); cursor: pointer; }
.spoiler-blur:hover { filter: blur(4px); }
.spoiler-blur.revealed { filter: none; }
.nav-links {
  margin-top: 12px;
}
.nav-links a {
  color: rgba(255,255,255,0.8);
  text-decoration: none;
  margin: 0 8px;
}
.nav-links a:hover { color: white; text-decoration: underline; }
.empty {
  text-align: center;
  padding: 60px 20px;
  color: #999;
  font-size: 18px;
}
</style>
</head>
<body>
<div class="header">
  <h1>川柳一覧</h1>
  <p>FindSenryu4Discord で検出された川柳</p>
  <div class="nav-links">
    <a href="/">一覧</a>
    <a href="/upload">背景画像アップロード</a>
  </div>
</div>
<div class="container">
  <div class="filter-bar">
    <input type="text" id="guildFilter" placeholder="サーバーIDでフィルタ">
    <button onclick="loadSenryus(1)">検索</button>
  </div>
  <div class="senryu-grid" id="senryuGrid"></div>
  <div class="pagination" id="pagination"></div>
</div>
<script>
let currentPage = 1;
const pageSize = 12;

async function loadSenryus(page) {
  currentPage = page;
  const guildId = document.getElementById('guildFilter').value;
  let url = '/api/senryu?page=' + page + '&page_size=' + pageSize;
  if (guildId) url += '&guild_id=' + encodeURIComponent(guildId);

  const resp = await fetch(url);
  const data = await resp.json();
  renderGrid(data.senryus || []);
  renderPagination(data.total, data.page, data.page_size);
}

function renderGrid(senryus) {
  const grid = document.getElementById('senryuGrid');
  if (senryus.length === 0) {
    grid.innerHTML = '<div class="empty">川柳が見つかりませんでした</div>';
    return;
  }
  grid.innerHTML = senryus.map(s => {
    const spoilerClass = s.spoiler ? 'spoiler-blur' : '';
    return '<div class="senryu-card">' +
      '<img src="' + s.image_url + '" class="' + spoilerClass + '" onclick="this.classList.toggle(\'revealed\')" loading="lazy">' +
      '<div class="info">' +
        '<div class="text">' + escHtml(s.kamigo + ' ' + s.nakasichi + ' ' + s.simogo) + '</div>' +
        '<div class="meta">#' + s.id + ' / ' + s.created_at + '</div>' +
      '</div>' +
    '</div>';
  }).join('');
}

function renderPagination(total, page, size) {
  const pages = Math.ceil(total / size);
  const el = document.getElementById('pagination');
  if (pages <= 1) { el.innerHTML = ''; return; }
  let html = '';
  if (page > 1) html += '<button onclick="loadSenryus(' + (page-1) + ')">前へ</button>';
  for (let i = Math.max(1, page-2); i <= Math.min(pages, page+2); i++) {
    html += '<button class="' + (i===page?'active':'') + '" onclick="loadSenryus(' + i + ')">' + i + '</button>';
  }
  if (page < pages) html += '<button onclick="loadSenryus(' + (page+1) + ')">次へ</button>';
  el.innerHTML = html;
}

function escHtml(s) {
  const d = document.createElement('div');
  d.textContent = s;
  return d.innerHTML;
}

loadSenryus(1);
</script>
</body>
</html>`

const uploadHTML = `<!DOCTYPE html>
<html lang="ja">
<head>
<meta charset="UTF-8">
<meta name="viewport" content="width=device-width, initial-scale=1.0">
<title>背景画像アップロード - FindSenryu</title>
<style>
* { margin: 0; padding: 0; box-sizing: border-box; }
body {
  font-family: 'Noto Sans JP', 'Hiragino Kaku Gothic ProN', sans-serif;
  background: #f5f0e8;
  color: #333;
  min-height: 100vh;
}
.header {
  background: linear-gradient(135deg, #8b4513, #a0522d);
  color: white;
  padding: 24px;
  text-align: center;
}
.header h1 { font-size: 1.8em; font-weight: 700; }
.header p { margin-top: 8px; opacity: 0.8; }
.nav-links { margin-top: 12px; }
.nav-links a {
  color: rgba(255,255,255,0.8);
  text-decoration: none;
  margin: 0 8px;
}
.nav-links a:hover { color: white; text-decoration: underline; }
.container {
  max-width: 600px;
  margin: 40px auto;
  padding: 24px;
}
.upload-form {
  background: white;
  border-radius: 12px;
  padding: 32px;
  box-shadow: 0 2px 8px rgba(0,0,0,0.1);
}
.form-group { margin-bottom: 20px; }
.form-group label {
  display: block;
  margin-bottom: 8px;
  font-weight: 600;
}
.form-group input[type="text"],
.form-group input[type="file"] {
  width: 100%;
  padding: 10px 14px;
  border: 1px solid #ccc;
  border-radius: 6px;
  font-size: 14px;
}
.form-group small { color: #888; margin-top: 4px; display: block; }
.submit-btn {
  width: 100%;
  padding: 12px;
  background: #8b4513;
  color: white;
  border: none;
  border-radius: 6px;
  font-size: 16px;
  cursor: pointer;
}
.submit-btn:hover { background: #a0522d; }
.submit-btn:disabled { background: #ccc; cursor: not-allowed; }
.message {
  margin-top: 16px;
  padding: 12px;
  border-radius: 6px;
  display: none;
}
.message.success { display: block; background: #d4edda; color: #155724; }
.message.error { display: block; background: #f8d7da; color: #721c24; }
.preview-area {
  margin-top: 16px;
  text-align: center;
}
.preview-area img {
  max-width: 100%;
  max-height: 300px;
  border-radius: 8px;
  border: 1px solid #ddd;
}
</style>
</head>
<body>
<div class="header">
  <h1>背景画像アップロード</h1>
  <p>川柳画像のカスタム背景を設定</p>
  <div class="nav-links">
    <a href="/">一覧</a>
    <a href="/upload">背景画像アップロード</a>
  </div>
</div>
<div class="container">
  <div class="upload-form">
    <form id="uploadForm" enctype="multipart/form-data">
      <div class="form-group">
        <label>サーバーID (Guild ID)</label>
        <input type="text" name="guild_id" id="guildId" required placeholder="例: 123456789012345678">
        <small>Discordサーバーの設定 → ウィジェットからIDをコピーできます</small>
      </div>
      <div class="form-group">
        <label>背景画像</label>
        <input type="file" name="image" id="imageFile" accept="image/*" required>
        <small>JPEG, PNG, GIF, WebP (最大10MB)。自動的にWebPに変換されます。</small>
      </div>
      <div class="preview-area" id="previewArea"></div>
      <button type="submit" class="submit-btn" id="submitBtn">アップロード</button>
    </form>
    <div class="message" id="message"></div>
  </div>
</div>
<script>
document.getElementById('imageFile').addEventListener('change', function(e) {
  const file = e.target.files[0];
  const area = document.getElementById('previewArea');
  if (file) {
    const reader = new FileReader();
    reader.onload = function(ev) {
      area.innerHTML = '<img src="' + ev.target.result + '">';
    };
    reader.readAsDataURL(file);
  } else {
    area.innerHTML = '';
  }
});

document.getElementById('uploadForm').addEventListener('submit', async function(e) {
  e.preventDefault();
  const btn = document.getElementById('submitBtn');
  const msg = document.getElementById('message');
  btn.disabled = true;
  btn.textContent = 'アップロード中...';
  msg.className = 'message';

  const formData = new FormData(this);
  try {
    const resp = await fetch('/api/background', { method: 'POST', body: formData });
    const data = await resp.json();
    if (resp.ok) {
      msg.className = 'message success';
      msg.textContent = data.message || 'アップロード成功';
    } else {
      msg.className = 'message error';
      msg.textContent = data.error || 'アップロードに失敗しました';
    }
  } catch (err) {
    msg.className = 'message error';
    msg.textContent = 'ネットワークエラー: ' + err.message;
  }
  btn.disabled = false;
  btn.textContent = 'アップロード';
});
</script>
</body>
</html>`
