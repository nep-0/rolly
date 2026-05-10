const api = async (path, options = {}) => {
  const res = await fetch(path, {
    headers: options.body instanceof FormData ? undefined : { "Content-Type": "application/json" },
    ...options,
  });
  if (!res.ok) {
    const text = await res.text();
    throw new Error(text || res.statusText);
  }
  const text = await res.text();
  return text ? JSON.parse(text) : null;
};

const el = (sel) => document.querySelector(sel);
const byId = (id) => document.getElementById(id);
const uid = () => `id-${Date.now()}-${Math.random().toString(16).slice(2, 8)}`;

const state = {
  models: [],
  cameras: [],
  stocks: [],
  activeTab: "models",
  activeStockId: "",
  activeImageRangeId: "",
  editModel: null,
  editCamera: null,
  editStock: null,
  editRange: null,
};

const forms = {
  model: byId("filmModelForm"),
  camera: byId("cameraForm"),
  stock: byId("stockForm"),
  range: byId("rangeForm"),
};

function toast(message, kind = "success") {
  const root = byId("toasts");
  const node = document.createElement("div");
  node.className = `toast ${kind}`;
  node.textContent = message;
  root.appendChild(node);
  setTimeout(() => node.remove(), 2800);
}

function openStockModal() {
  byId("stockModal").classList.remove("hidden");
}

function closeStockModal() {
  byId("stockModal").classList.add("hidden");
}

function setTab(name) {
  state.activeTab = name;
  document.querySelectorAll("[data-tab]").forEach((btn) => {
    btn.classList.toggle("active", btn.getAttribute("data-tab") === name);
  });
  document.querySelectorAll("[data-panel]").forEach((panel) => {
    panel.classList.toggle("hidden", panel.getAttribute("data-panel") !== name);
  });
}

function fillSelect(node, rows, placeholder, labelFn) {
  node.innerHTML = [`<option value="">${placeholder}</option>`]
    .concat(rows.map((row) => `<option value="${row.id}">${labelFn(row)}</option>`))
    .join("");
}

function generateId(prefix) {
  return `${prefix}-${crypto.randomUUID().slice(0, 8)}`;
}

function toDatetimeLocal(value) {
  if (!value) return "";
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return "";
  const local = new Date(date.getTime() - date.getTimezoneOffset() * 60000);
  return local.toISOString().slice(0, 19);
}

function fromDatetimeLocal(value) {
  if (!value) return undefined;
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return undefined;
  return date.toISOString();
}

function imageContentURL(imageId) {
  return `/api/v1/images/${encodeURIComponent(imageId)}/content`;
}

function rangeColor(index) {
  const colors = ["#58cc02", "#1cb0f6", "#ff9600", "#ce82ff", "#ffc800", "#ff4b4b", "#89e219", "#2b70c9"];
  return colors[index % colors.length];
}

async function refreshAll() {
  const [models, cameras, stocks] = await Promise.all([
    api("/api/v1/film-models"),
    api("/api/v1/cameras"),
    api("/api/v1/film-stocks"),
  ]);
  state.models = models || [];
  state.cameras = cameras || [];
  state.stocks = stocks || [];
  renderCollections();
  syncEditors();
  syncStockTargets();
  if (!state.activeStockId && state.stocks[0]) {
    state.activeStockId = state.stocks[0].id;
  }
  if (state.activeStockId) {
    await loadStockDetail(state.activeStockId);
  } else {
    byId("stockDetailPane").innerHTML = `<div class="empty">No stock selected.</div>`;
  }
}

function renderCollections() {
  byId("filmModels").innerHTML = state.models.map((m) => `
    <article class="card">
      <div class="card-head">
        <div>
          <div class="card-title">${m.name}</div>
          <div class="card-sub"><code>${m.id}</code> ISO ${m.iso} | ${m.size}</div>
        </div>
        <div class="card-actions">
          <button type="button" class="secondary" data-edit-model="${m.id}">Edit</button>
        </div>
      </div>
    </article>
  `).join("");

  byId("cameras").innerHTML = state.cameras.map((c) => `
    <article class="card">
      <div class="card-head">
        <div>
          <div class="card-title">${c.name}</div>
          <div class="card-sub"><code>${c.id}</code> ${c.maker} ${c.model}</div>
        </div>
        <div class="card-actions">
          <button type="button" class="secondary" data-edit-camera="${c.id}">Edit</button>
        </div>
      </div>
    </article>
  `).join("");

  byId("stocks").innerHTML = state.stocks.map((s) => `
    <article class="card">
      <div class="card-head">
        <div>
          <div class="card-title">${s.id}</div>
          <div class="card-sub">${s.model_id} | ${s.camera_id} | ${s.expiry_year}-${String(s.expiry_month).padStart(2, "0")}</div>
          <div class="card-sub">${s.comment || ""}</div>
        </div>
        <div class="card-actions">
          <button type="button" class="secondary" data-open-stock="${s.id}">Manage</button>
          <button type="button" class="secondary" data-edit-stock="${s.id}">Edit</button>
        </div>
      </div>
    </article>
  `).join("");

  byId("filmModels").querySelectorAll("[data-edit-model]").forEach((btn) => {
    btn.onclick = () => loadModelEditor(btn.getAttribute("data-edit-model"));
  });
  byId("cameras").querySelectorAll("[data-edit-camera]").forEach((btn) => {
    btn.onclick = () => loadCameraEditor(btn.getAttribute("data-edit-camera"));
  });
  byId("stocks").querySelectorAll("[data-open-stock]").forEach((btn) => {
    btn.onclick = async () => {
      state.activeStockId = btn.getAttribute("data-open-stock");
      setTab("stocks");
      await loadStockDetail(state.activeStockId);
      openStockModal();
    };
  });
  byId("stocks").querySelectorAll("[data-edit-stock]").forEach((btn) => {
    btn.onclick = async () => {
      state.activeStockId = btn.getAttribute("data-edit-stock");
      setTab("stocks");
      loadStockEditor(state.stocks.find((s) => s.id === state.activeStockId));
      await loadStockDetail(state.activeStockId);
    };
  });
}

function syncEditors() {
  if (!state.editModel) {
    clearModelEditor();
  } else {
    loadModelEditor(state.editModel.id);
  }
  if (!state.editCamera) {
    clearCameraEditor();
  } else {
    loadCameraEditor(state.editCamera.id);
  }
  if (!state.editStock) {
    loadStockEditor(null);
  } else {
    loadStockEditor(state.editStock);
  }
  if (!state.editRange) {
    loadRangeEditor(null);
  } else {
    loadRangeEditor(state.editRange);
  }
}

function syncStockTargets() {
  fillSelect(byId("stockModelSelect"), state.models, "Choose model", (m) => `${m.name} (${m.id})`);
  fillSelect(byId("stockCameraSelect"), state.cameras, "Choose camera", (c) => `${c.name} (${c.id})`);
  fillSelect(byId("rangeStockSelect"), state.stocks, "Choose stock", (s) => `${s.id}`);
  if (state.activeStockId) {
    byId("rangeStockSelect").value = state.activeStockId;
  }
}

function clearModelEditor() {
  state.editModel = null;
  byId("modelFormTitle").textContent = "Create film model";
  byId("modelFormMode").textContent = "New";
  forms.model.reset();
  forms.model.elements.id.value = generateId("film");
}

function loadModelEditor(id) {
  const model = state.models.find((x) => x.id === id);
  if (!model) return clearModelEditor();
  state.editModel = model;
  byId("modelFormTitle").textContent = "Edit film model";
  byId("modelFormMode").textContent = "Edit";
  forms.model.elements.id.value = model.id;
  forms.model.elements.name.value = model.name;
  forms.model.elements.iso.value = model.iso;
  forms.model.elements.size.value = model.size;
  forms.model.elements.nominal_photo_count.value = model.nominal_photo_count ?? "";
  forms.model.elements.supported_processing.value = (model.supported_processing || []).join(", ");
}

function clearCameraEditor() {
  state.editCamera = null;
  byId("cameraFormTitle").textContent = "Create camera";
  byId("cameraFormMode").textContent = "New";
  forms.camera.reset();
  forms.camera.elements.id.value = generateId("cam");
}

function loadCameraEditor(id) {
  const camera = state.cameras.find((x) => x.id === id);
  if (!camera) return clearCameraEditor();
  state.editCamera = camera;
  byId("cameraFormTitle").textContent = "Edit camera";
  byId("cameraFormMode").textContent = "Edit";
  forms.camera.elements.id.value = camera.id;
  forms.camera.elements.name.value = camera.name;
  forms.camera.elements.maker.value = camera.maker;
  forms.camera.elements.model.value = camera.model;
  forms.camera.elements.serial_number.value = camera.serial_number;
  forms.camera.elements.metering_mode.value = camera.metering_mode;
  forms.camera.elements.focal_length.value = camera.focal_length;
  forms.camera.elements.focal_length_35mm.value = camera.focal_length_35mm;
}

function loadStockEditor(stock) {
  if (!stock) {
    state.editStock = null;
    byId("stockFormTitle").textContent = "Create stock";
    byId("stockFormMode").textContent = "New";
    forms.stock.reset();
    forms.stock.elements.id.value = generateId("stk");
    forms.stock.elements.model_id.value = "";
    forms.stock.elements.camera_id.value = "";
    return;
  }
  state.editStock = stock;
  byId("stockFormTitle").textContent = "Edit stock";
  byId("stockFormMode").textContent = "Edit";
  forms.stock.elements.id.value = stock.id;
  forms.stock.elements.model_id.value = stock.model_id;
  forms.stock.elements.camera_id.value = stock.camera_id;
  forms.stock.elements.expiry_year.value = stock.expiry_year;
  forms.stock.elements.expiry_month.value = stock.expiry_month;
  forms.stock.elements.emulsion_number.value = stock.emulsion_number;
  forms.stock.elements.chosen_processing.value = stock.chosen_processing;
  forms.stock.elements.scanner_model.value = stock.scanner_model;
  forms.stock.elements.comment.value = stock.comment || "";
}

function loadRangeEditor(range) {
  if (!range) {
    state.editRange = null;
    byId("rangeFormTitle").textContent = "Create range";
    byId("rangeFormMode").textContent = "New";
    forms.range.reset();
    forms.range.elements.id.value = generateId("rng");
    forms.range.elements.stock_id.value = state.activeStockId || "";
    return;
  }
  state.editRange = range;
  byId("rangeFormTitle").textContent = "Edit range";
  byId("rangeFormMode").textContent = "Edit";
  forms.range.elements.id.value = range.id;
  forms.range.elements.stock_id.value = range.stock_id;
  forms.range.elements.start_frame.value = range.start_frame;
  forms.range.elements.end_frame.value = range.end_frame;
  forms.range.elements.shot_from.value = toDatetimeLocal(range.shot_from);
  forms.range.elements.shot_to.value = toDatetimeLocal(range.shot_to);
  forms.range.elements.location.value = range.location || "";
  forms.range.elements.weather.value = range.weather || "";
  forms.range.elements.notes.value = range.notes || "";
}

async function loadStockDetail(stockId) {
  if (!stockId) return;
  const detail = await api(`/api/v1/film-stocks/${stockId}`);
  state.activeStockId = stockId;
  byId("rangeStockSelect").value = stockId;
  const ranges = detail.ranges || [];
  const images = detail.images || [];
  if (state.activeImageRangeId && !ranges.some((r) => r.id === state.activeImageRangeId)) {
    state.activeImageRangeId = "";
  }
  byId("stockDetailPane").innerHTML = `
    <div class="detail-grid">
      <div class="detail-top">
        <div>
          <div class="meta">Stock</div>
          <div class="detail-title">${detail.stock.id}</div>
          <div class="small">${detail.model.name} | ${detail.camera.name}</div>
          <div class="small">${detail.stock.comment || ""}</div>
        </div>
        <div class="card-actions">
          <button id="exportBtn" type="button">Export</button>
        </div>
      </div>

      <div class="subpanel">
        <h3>Ranges</h3>
        <div id="rangeList"></div>
      </div>

      <div class="subpanel">
        <h3>Images</h3>
        <div class="image-toolbar">
          <div class="card-actions">
            <input id="uploadFiles" type="file" multiple>
            <button id="uploadBtn" type="button">Upload</button>
          </div>
        </div>
        <div class="range-paint">
          <div>
            <div class="section-label">Assign by color</div>
            <div class="small">Select a range, then click images to color or uncolor them.</div>
          </div>
          <div id="rangePaintPicker" class="range-chip-list">
            ${ranges.length ? ranges.map((r, index) => `
              <button
                type="button"
                class="range-chip ${state.activeImageRangeId === r.id ? "active" : ""}"
                data-pick-range="${r.id}"
                style="--range-color: ${rangeColor(index)}"
              >
                <span class="range-dot"></span>
                <span>${r.start_frame}-${r.end_frame}</span>
              </button>
            `).join("") : `<span class="empty-inline">Create a range before assigning images.</span>`}
          </div>
        </div>
        <div id="imageList"></div>
      </div>
    </div>
  `;

  byId("rangeList").innerHTML = ranges.map((r, index) => `
    <div class="range-row">
      <div>
        <div class="card-title">${r.start_frame}-${r.end_frame}</div>
        <div class="card-sub">${r.location || ""} ${r.weather || ""}</div>
        <div class="card-sub">${r.notes || ""}</div>
      </div>
      <div class="card-actions">
        <span class="range-swatch" style="--range-color: ${rangeColor(index)}"></span>
        <button type="button" class="secondary" data-edit-range="${r.id}">Edit</button>
      </div>
    </div>
  `).join("");

  renderImages(stockId, ranges, images);

  byId("rangeList").querySelectorAll("[data-edit-range]").forEach((btn) => {
    btn.onclick = () => {
      const range = ranges.find((r) => r.id === btn.getAttribute("data-edit-range"));
      if (range) loadRangeEditor(range);
    };
  });

  byId("rangePaintPicker").querySelectorAll("[data-pick-range]").forEach((btn) => {
    btn.onclick = () => {
      const rangeId = btn.getAttribute("data-pick-range");
      state.activeImageRangeId = state.activeImageRangeId === rangeId ? "" : rangeId;
      syncActiveRangeChips();
    };
  });

  byId("uploadBtn").onclick = async () => {
    const files = byId("uploadFiles").files;
    const fd = new FormData();
    for (const file of files) fd.append("files", file);
    await api(`/api/v1/film-stocks/${stockId}/images`, { method: "POST", body: fd });
    toast("Images uploaded");
    await loadStockDetail(stockId);
    await refreshAll();
  };

  byId("exportBtn").onclick = async () => {
    await api(`/api/v1/film-stocks/${stockId}/exports`, { method: "POST" });
    toast("Export complete");
  };
}

function syncActiveRangeChips() {
  byId("rangePaintPicker")?.querySelectorAll("[data-pick-range]").forEach((btn) => {
    btn.classList.toggle("active", btn.getAttribute("data-pick-range") === state.activeImageRangeId);
  });
}

function updateImageTileRange(tile, rangeData) {
  const badge = tile.querySelector(".image-range-badge");
  if (!rangeData) {
    tile.classList.remove("assigned");
    tile.dataset.rangeId = "";
    tile.style.setProperty("--range-color", "transparent");
    badge?.remove();
    return;
  }

  tile.classList.add("assigned");
  tile.dataset.rangeId = rangeData.range.id;
  tile.style.setProperty("--range-color", rangeData.color);
  const text = `${rangeData.range.start_frame}-${rangeData.range.end_frame}`;
  if (badge) {
    badge.textContent = text;
    return;
  }
  const node = document.createElement("span");
  node.className = "image-range-badge";
  node.textContent = text;
  tile.querySelector(".thumb").appendChild(node);
}

function renumberImageTiles(root) {
  root.querySelectorAll("[data-image-id]").forEach((tile, index) => {
    const order = tile.querySelector(".image-caption strong");
    if (order) order.textContent = String(index + 1);
  });
}

function moveImageTile(root, movedId, targetId, from, to) {
  const draggedTile = [...root.querySelectorAll("[data-image-id]")].find((node) => node.dataset.imageId === movedId);
  const targetTile = [...root.querySelectorAll("[data-image-id]")].find((node) => node.dataset.imageId === targetId);
  if (!draggedTile || !targetTile) return;
  if (from < to) {
    targetTile.after(draggedTile);
  } else {
    targetTile.before(draggedTile);
  }
  renumberImageTiles(root);
}

function renderImages(stockId, ranges, images) {
  const root = byId("imageList");
  const ordered = [...images].sort((a, b) => a.order_index - b.order_index);
  const rangeMap = new Map(ranges.map((range, index) => [range.id, { range, color: rangeColor(index) }]));
  root.className = "image-grid";
  root.innerHTML = ordered.map((img) => {
    const assigned = img.range_id ? rangeMap.get(img.range_id) : null;
    return `
      <div
        class="image-tile ${assigned ? "assigned" : ""}"
        draggable="true"
        data-image-id="${img.id}"
        data-range-id="${img.range_id || ""}"
        style="--range-color: ${assigned ? assigned.color : "transparent"}"
      >
        <div class="thumb">
          <img src="${imageContentURL(img.id)}" alt="${img.original_name}" loading="lazy">
          ${assigned ? `<span class="image-range-badge">${assigned.range.start_frame}-${assigned.range.end_frame}</span>` : ""}
        </div>
        <div class="image-caption">
          <strong>${img.order_index}</strong>
          <span>${img.original_name}</span>
        </div>
      </div>
    `;
  }).join("");

  let draggedId = "";
  let didDrag = false;
  root.querySelectorAll("[data-image-id]").forEach((tile) => {
    tile.addEventListener("dragstart", (event) => {
      draggedId = tile.getAttribute("data-image-id");
      didDrag = true;
      event.dataTransfer.effectAllowed = "move";
    });
    tile.addEventListener("dragover", (event) => {
      event.preventDefault();
      tile.classList.add("drag-over");
    });
    tile.addEventListener("dragleave", () => tile.classList.remove("drag-over"));
    tile.addEventListener("drop", async (event) => {
      event.preventDefault();
      tile.classList.remove("drag-over");
      const targetId = tile.getAttribute("data-image-id");
      if (!draggedId || draggedId === targetId) return;
      const movedId = draggedId;
      const nextOrder = [...root.querySelectorAll("[data-image-id]")].map((node) => node.getAttribute("data-image-id"));
      const from = nextOrder.indexOf(movedId);
      const to = nextOrder.indexOf(targetId);
      if (from < 0 || to < 0) return;
      const [moved] = nextOrder.splice(from, 1);
      nextOrder.splice(to, 0, moved);
      await api(`/api/v1/film-stocks/${stockId}/images/reorder`, {
        method: "PATCH",
        body: JSON.stringify({ image_ids: nextOrder }),
      });
      moveImageTile(root, movedId, targetId, from, to);
      toast("Image order updated");
    });
    tile.addEventListener("dragend", () => {
      setTimeout(() => {
        didDrag = false;
        draggedId = "";
      }, 0);
    });
    tile.addEventListener("click", async () => {
      if (didDrag || !state.activeImageRangeId) return;
      const imageId = tile.getAttribute("data-image-id");
      const currentRangeId = tile.getAttribute("data-range-id");
      const nextRangeId = currentRangeId === state.activeImageRangeId ? "" : state.activeImageRangeId;
      await api(`/api/v1/images/${imageId}/range`, {
        method: "PATCH",
        body: JSON.stringify({ range_id: nextRangeId }),
      });
      updateImageTileRange(tile, nextRangeId ? rangeMap.get(nextRangeId) : null);
      toast(nextRangeId ? "Range assigned" : "Range removed");
    });
  });
}

forms.model.addEventListener("submit", async (e) => {
  e.preventDefault();
  const fd = new FormData(forms.model);
  const id = fd.get("id") || generateId("film");
  const editing = Boolean(state.editModel);
  await api(editing ? `/api/v1/film-models/${encodeURIComponent(id)}` : "/api/v1/film-models", {
    method: editing ? "PUT" : "POST",
    body: JSON.stringify({
      id,
      name: fd.get("name"),
      iso: Number(fd.get("iso")),
      size: fd.get("size"),
      nominal_photo_count: fd.get("nominal_photo_count") ? Number(fd.get("nominal_photo_count")) : undefined,
      supported_processing: String(fd.get("supported_processing") || "").split(",").map((s) => s.trim()).filter(Boolean),
    }),
  });
  toast("Film model saved");
  clearModelEditor();
  await refreshAll();
});

forms.camera.addEventListener("submit", async (e) => {
  e.preventDefault();
  const fd = new FormData(forms.camera);
  const id = fd.get("id") || generateId("cam");
  const editing = Boolean(state.editCamera);
  await api(editing ? `/api/v1/cameras/${encodeURIComponent(id)}` : "/api/v1/cameras", {
    method: editing ? "PUT" : "POST",
    body: JSON.stringify({
      id,
      name: fd.get("name"),
      maker: fd.get("maker"),
      model: fd.get("model"),
      serial_number: fd.get("serial_number"),
      metering_mode: fd.get("metering_mode"),
      focal_length: fd.get("focal_length") ? Number(fd.get("focal_length")) : 0,
      focal_length_35mm: fd.get("focal_length_35mm") ? Number(fd.get("focal_length_35mm")) : 0,
    }),
  });
  toast("Camera saved");
  clearCameraEditor();
  await refreshAll();
});

forms.stock.addEventListener("submit", async (e) => {
  e.preventDefault();
  const fd = new FormData(forms.stock);
  const id = fd.get("id") || generateId("stk");
  const editing = Boolean(state.editStock);
  await api(editing ? `/api/v1/film-stocks/${encodeURIComponent(id)}` : "/api/v1/film-stocks", {
    method: editing ? "PUT" : "POST",
    body: JSON.stringify({
      id,
      model_id: fd.get("model_id"),
      camera_id: fd.get("camera_id"),
      expiry_year: Number(fd.get("expiry_year")),
      expiry_month: Number(fd.get("expiry_month")),
      emulsion_number: fd.get("emulsion_number"),
      chosen_processing: fd.get("chosen_processing"),
      scanner_model: fd.get("scanner_model"),
      comment: fd.get("comment") || "",
    }),
  });
  toast("Stock saved");
  state.activeStockId = id;
  loadStockEditor(state.stocks.find((s) => s.id === id) || {
    id,
    model_id: fd.get("model_id"),
    camera_id: fd.get("camera_id"),
    expiry_year: Number(fd.get("expiry_year")),
    expiry_month: Number(fd.get("expiry_month")),
    emulsion_number: fd.get("emulsion_number"),
    chosen_processing: fd.get("chosen_processing"),
    scanner_model: fd.get("scanner_model"),
    comment: fd.get("comment") || "",
  });
  await refreshAll();
});

forms.range.addEventListener("submit", async (e) => {
  e.preventDefault();
  const fd = new FormData(forms.range);
  const id = fd.get("id") || generateId("rng");
  const stockId = fd.get("stock_id") || state.activeStockId;
  const editing = Boolean(state.editRange);
  await api(editing ? `/api/v1/film-stocks/${encodeURIComponent(stockId)}/ranges/${encodeURIComponent(id)}` : `/api/v1/film-stocks/${encodeURIComponent(stockId)}/ranges`, {
    method: editing ? "PUT" : "POST",
    body: JSON.stringify({
      id,
      stock_id: stockId,
      start_frame: Number(fd.get("start_frame")),
      end_frame: Number(fd.get("end_frame")),
      shot_from: fromDatetimeLocal(fd.get("shot_from")),
      shot_to: fromDatetimeLocal(fd.get("shot_to")),
      location: fd.get("location") || "",
      weather: fd.get("weather") || "",
      notes: fd.get("notes") || "",
    }),
  });
  toast("Range saved");
  loadRangeEditor(null);
  await loadStockDetail(stockId);
});

function clearAllEditors() {
  clearModelEditor();
  clearCameraEditor();
  loadStockEditor(null);
  loadRangeEditor(null);
}

byId("newModelBtn").addEventListener("click", clearModelEditor);
byId("newCameraBtn").addEventListener("click", clearCameraEditor);
byId("newStockBtn").addEventListener("click", () => {
  loadStockEditor(null);
});
byId("newRangeBtn").addEventListener("click", () => {
  loadRangeEditor(null);
});
document.querySelectorAll("[data-close-stock-modal]").forEach((node) => {
  node.addEventListener("click", closeStockModal);
});

document.querySelectorAll("[data-tab]").forEach((btn) => {
  btn.onclick = () => setTab(btn.getAttribute("data-tab"));
});

refreshAll().then(() => {
  setTab("models");
}).catch((err) => {
  console.error(err);
  toast(err.message, "error");
});
