const API_BASE = '/api';
const API_KEY = ''; // for demo, can set in header via prompt or include in .env for server

async function fetchJSON(url, opts) {
  const res = await fetch(url, opts);
  const text = await res.text();
  try {
    return { ok: res.ok, status: res.status, data: JSON.parse(text), text: text };
  } catch (e) {
    return { ok: res.ok, status: res.status, data: null, text: text };
  }
}

async function initUpload(filename, method) {
  const res = await fetch(`${API_BASE}/uploads/init`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json', 'X-API-KEY': API_KEY },
    body: JSON.stringify({ filename, method })
  });
  if (!res.ok) {
    return { error: await res.text() };
  }
  return res.json();
}

async function createPost(title, description, objectKey) {
  const res = await fetch(`${API_BASE}/posts`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json', 'X-API-KEY': API_KEY },
    body: JSON.stringify({ title, description, object_key: objectKey })
  });
  return res;
}

async function uploadFileUsingPost(url, fields, file) {
  const fd = new FormData();
  // add fields first
  Object.entries(fields).forEach(([k, v]) => fd.append(k, v));
  fd.append('file', file); // MinIO expects 'file' field or 'file' key depending on policy; we'll use standard
  const res = await fetch(url, { method: 'POST', body: fd });
  return res;
}

async function uploadFileUsingPut(url, file) {
  const res = await fetch(url, { method: 'PUT', body: file, headers: { 'Content-Type': file.type } });
  return res;
}

async function handleFormSubmit(e) {
  e.preventDefault();
  const title = document.getElementById('title').value.trim();
  const description = document.getElementById('description').value.trim();
  const file = document.getElementById('media').files[0];
  const method = document.getElementById('method').value;

  const msg = document.getElementById('message');
  if (!title || !file) {
    msg.innerText = 'Title and media required';
    return;
  }
  if (file.size > 10 * 1024 * 1024) {
    msg.innerText = 'File too large (max 10MB)';
    return;
  }
  msg.innerText = 'Requesting presigned info...';

  const info = await initUpload(file.name, method);
  if (info.error) {
    msg.innerText = 'Init failed: ' + info.error;
    return;
  }

  if (info.method === 'post') {
    // fields + url
    const res = await uploadFileUsingPost(info.url, info.fields, file);
    if (!res.ok) {
      msg.innerText = 'Upload failed: ' + (await res.text());
      return;
    }
  } else if (info.method === 'put') {
    const res = await uploadFileUsingPut(info.url, file);
    if (!res.ok) {
      msg.innerText = 'Upload failed (PUT): ' + (await res.text());
      return;
    }
  }

  // Confirm to server to create post record
  const createRes = await createPost(title, description, info.objectKey);
  if (createRes.ok) {
    msg.innerText = 'Post created!';
    document.getElementById('preForm').reset();
    renderPosts();
  } else {
    msg.innerText = 'Create post failed: ' + await createRes.text();
  }
}

async function fetchPosts() {
  const res = await fetch(`${API_BASE}/posts`);
  if (!res.ok) return [];
  return res.json();
}

function createCard(post) {
  const div = document.createElement('div');
  div.className = 'bg-white p-3 rounded shadow';
  const title = document.createElement('h3');
  title.className = 'font-semibold';
  title.innerText = post.title;
  div.appendChild(title);

  if (post.media_type === 'image') {
    const img = document.createElement('img');
    img.src = post.media_url;
    img.className = 'mt-2 max-h-48 w-full object-cover rounded';
    div.appendChild(img);
  } else if (post.media_type === 'video') {
    const vid = document.createElement('video');
    vid.src = post.media_url;
    vid.controls = true;
    vid.className = 'mt-2 max-h-48 w-full object-cover rounded';
    div.appendChild(vid);
  }

  if (post.description) {
    const p = document.createElement('p');
    p.className = 'mt-2 text-sm text-gray-700';
    p.innerText = post.description;
    div.appendChild(p);
  }

  const meta = document.createElement('div');
  meta.className = 'mt-2 flex justify-between items-center';
  const time = document.createElement('small');
  time.className = 'text-xs text-gray-500';
  time.innerText = new Date(post.created_at).toLocaleString();
  meta.appendChild(time);

  const del = document.createElement('button');
  del.className = 'text-sm text-red-600';
  del.innerText = 'Delete';
  del.onclick = async () => {
    if (!confirm('Delete post?')) return;
    const res = await fetch(`${API_BASE}/posts/${post.id}`, { method: 'DELETE', headers: { 'X-API-KEY': API_KEY } });
    if (res.ok) {
      div.remove();
    } else {
      alert('Delete failed');
    }
  };
  meta.appendChild(del);

  div.appendChild(meta);
  return div;
}

async function renderPosts() {
  const container = document.getElementById('posts');
  container.innerHTML = '';
  const posts = await fetchPosts();
  posts.forEach(p => container.appendChild(createCard(p)));
}

document.getElementById('preForm').addEventListener('submit', handleFormSubmit);
window.addEventListener('load', () => renderPosts());
