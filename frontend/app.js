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

const renderList = (node, items, renderItem) => {
  node.innerHTML = (items || []).map(renderItem).join("");
};

const filmModelForm = el("#filmModelForm");
const cameraForm = el("#cameraForm");
const stockForm = el("#stockForm");
const stockPickForm = el("#stockPickForm");

async function refreshAll() {
  const [models, cameras, stocks] = await Promise.all([
    api("/api/v1/film-models"),
    api("/api/v1/cameras"),
    api("/api/v1/film-stocks"),
  ]);

  renderList(el("#filmModels"), models, (m) => `
    <div class="item">
      <strong>${m.name}</strong> <code>${m.id}</code>
      <div class="small">ISO ${m.iso} | ${m.size} | ${m.supported_processing?.join(", ") || ""}</div>
    </div>
  `);

  renderList(el("#cameras"), cameras, (c) => `
    <div class="item">
      <strong>${c.name}</strong> <code>${c.id}</code>
      <div class="small">${c.maker} ${c.model} | ${c.serial_number}</div>
    </div>
  `);

  renderList(el("#stocks"), stocks, (s) => `
    <div class="item">
      <strong>${s.id}</strong>
      <div class="small">model ${s.model_id} | camera ${s.camera_id} | ${s.expiry_year}-${String(s.expiry_month).padStart(2, "0")}</div>
      <div class="small">${s.comment || ""}</div>
    </div>
  `);
}

filmModelForm.addEventListener("submit", async (e) => {
  e.preventDefault();
  const fd = new FormData(filmModelForm);
  await api("/api/v1/film-models", {
    method: "POST",
    body: JSON.stringify({
      id: fd.get("id") || undefined,
      name: fd.get("name"),
      iso: Number(fd.get("iso")),
      size: fd.get("size"),
      nominal_photo_count: fd.get("nominal_photo_count") ? Number(fd.get("nominal_photo_count")) : undefined,
      supported_processing: String(fd.get("supported_processing") || "").split(",").map((s) => s.trim()).filter(Boolean),
    }),
  });
  filmModelForm.reset();
  await refreshAll();
});

cameraForm.addEventListener("submit", async (e) => {
  e.preventDefault();
  const fd = new FormData(cameraForm);
  await api("/api/v1/cameras", {
    method: "POST",
    body: JSON.stringify(Object.fromEntries(fd.entries())),
  });
  cameraForm.reset();
  await refreshAll();
});

stockForm.addEventListener("submit", async (e) => {
  e.preventDefault();
  const fd = new FormData(stockForm);
  await api("/api/v1/film-stocks", {
    method: "POST",
    body: JSON.stringify({
      id: fd.get("id") || undefined,
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
  stockForm.reset();
  await refreshAll();
});

stockPickForm.addEventListener("submit", async (e) => {
  e.preventDefault();
  await loadStockDetail(new FormData(stockPickForm).get("stock_id"));
});

el("#refreshBtn").addEventListener("click", refreshAll);

async function loadStockDetail(stockId) {
  const detail = await api(`/api/v1/film-stocks/${stockId}`);
  const box = el("#stockDetail");
  box.innerHTML = `
    <div class="panel">
      <div><strong>Stock</strong> <code>${detail.stock.id}</code></div>
      <div class="small">Model ${detail.model.name} | Camera ${detail.camera.name}</div>
      <div class="small">${detail.stock.comment || ""}</div>
      <div class="small">Upload images, assign ranges, and export from the API endpoints.</div>
      <div class="actions">
        <input id="uploadFiles" type="file" multiple>
        <button id="uploadBtn">Upload</button>
        <button id="exportBtn">Export</button>
      </div>
      <form id="rangeForm" class="form" style="margin-top:12px">
        <input name="id" placeholder="Range ID">
        <input name="start_frame" type="number" placeholder="Start frame" required>
        <input name="end_frame" type="number" placeholder="End frame" required>
        <input name="shot_from" placeholder="Shot from RFC3339">
        <input name="shot_to" placeholder="Shot to RFC3339">
        <input name="location" placeholder="Location">
        <input name="weather" placeholder="Weather">
        <input name="notes" placeholder="Notes">
        <button type="submit">Add Range</button>
      </form>
      <div class="panel">
        <h3>Ranges</h3>
        <div class="list" id="rangesList"></div>
      </div>
      <div class="panel">
        <h3>Images</h3>
        <div class="list" id="imagesList"></div>
      </div>
    </div>
  `;

  const ranges = detail.ranges || [];
  const images = detail.images || [];
  el("#rangesList").innerHTML = ranges.map((r) => `
    <div class="item">
      <strong>${r.start_frame}-${r.end_frame}</strong>
      <div class="small">${r.location || ""} ${r.weather || ""}</div>
      <div class="small">${r.notes || ""}</div>
    </div>
  `).join("");
  el("#imagesList").innerHTML = images.map((img) => `
    <div class="item">
      <strong>${img.original_name}</strong> <code>${img.id}</code>
      <div class="small">order ${img.order_index} ${img.range_id ? `| range ${img.range_id}` : ""}</div>
      <div class="actions">
        <input data-range-for="${img.id}" placeholder="Range ID">
        <button data-assign-for="${img.id}">Assign range</button>
        <button data-move-up="${img.id}">Up</button>
        <button data-move-down="${img.id}">Down</button>
      </div>
    </div>
  `).join("");

  const rangeForm = box.querySelector("#rangeForm");
  if (rangeForm) {
    rangeForm.addEventListener("submit", async (e) => {
      e.preventDefault();
      const fd = new FormData(rangeForm);
      await api(`/api/v1/film-stocks/${stockId}/ranges`, {
        method: "POST",
        body: JSON.stringify({
          id: fd.get("id") || undefined,
          start_frame: Number(fd.get("start_frame")),
          end_frame: Number(fd.get("end_frame")),
          shot_from: fd.get("shot_from") || undefined,
          shot_to: fd.get("shot_to") || undefined,
          location: fd.get("location") || "",
          weather: fd.get("weather") || "",
          notes: fd.get("notes") || "",
        }),
      });
      await loadStockDetail(stockId);
    });
  }

  document.querySelectorAll("[data-assign-for]").forEach((btn) => {
    btn.onclick = async (e) => {
      e.preventDefault();
      const imgId = btn.getAttribute("data-assign-for");
      const rangeId = document.querySelector(`[data-range-for="${imgId}"]`).value;
      await api(`/api/v1/images/${imgId}/range`, {
        method: "PATCH",
        body: JSON.stringify({ range_id: rangeId }),
      });
      await loadStockDetail(stockId);
    };
  });

  const ordered = [...images].sort((a, b) => a.order_index - b.order_index);
  document.querySelectorAll("[data-move-up]").forEach((btn) => {
    btn.onclick = async (e) => {
      e.preventDefault();
      const imgId = btn.getAttribute("data-move-up");
      const idx = ordered.findIndex((img) => img.id === imgId);
      if (idx <= 0) return;
      const nextOrder = ordered.map((img) => img.id);
      [nextOrder[idx - 1], nextOrder[idx]] = [nextOrder[idx], nextOrder[idx - 1]];
      await api(`/api/v1/film-stocks/${stockId}/images/reorder`, {
        method: "PATCH",
        body: JSON.stringify({ image_ids: nextOrder }),
      });
      await loadStockDetail(stockId);
    };
  });

  document.querySelectorAll("[data-move-down]").forEach((btn) => {
    btn.onclick = async (e) => {
      e.preventDefault();
      const imgId = btn.getAttribute("data-move-down");
      const idx = ordered.findIndex((img) => img.id === imgId);
      if (idx < 0 || idx >= ordered.length - 1) return;
      const nextOrder = ordered.map((img) => img.id);
      [nextOrder[idx], nextOrder[idx + 1]] = [nextOrder[idx + 1], nextOrder[idx]];
      await api(`/api/v1/film-stocks/${stockId}/images/reorder`, {
        method: "PATCH",
        body: JSON.stringify({ image_ids: nextOrder }),
      });
      await loadStockDetail(stockId);
    };
  });

  el("#uploadBtn").onclick = async () => {
    const files = el("#uploadFiles").files;
    const fd = new FormData();
    for (const file of files) fd.append("files", file);
    await api(`/api/v1/film-stocks/${stockId}/images`, { method: "POST", body: fd });
    await loadStockDetail(stockId);
  };

  el("#exportBtn").onclick = async () => {
    await api(`/api/v1/film-stocks/${stockId}/exports`, { method: "POST" });
    alert("Export done");
  };
}

refreshAll().catch((err) => {
  console.error(err);
  alert(err.message);
});
