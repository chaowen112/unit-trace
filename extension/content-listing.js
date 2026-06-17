// Runs on PropertyGuru listing pages.
// Extracts listing data, submits to UnitTrace backend, and renders overlay.

(function () {
  "use strict";

  // --- Listing ID from URL ---
  // Supports:
  //   /property-listing/12345678-slug  → ID at start after slash
  //   /listing/hdb-for-sale-slug-12345678  → ID is last numeric segment

  function extractListingId() {
    const path = location.pathname;
    // Old format: /property-listing/12345678...
    let m = path.match(/\/property-listing\/(\d+)/);
    if (m) return m[1];
    // New format: /listing/...-12345678  (trailing digits)
    m = path.match(/\/listing\/.*?-(\d{6,})(?:[^0-9]|$)/);
    if (m) return m[1];
    // Fallback: last numeric segment of any length >= 6
    m = path.match(/(\d{6,})(?:[^0-9/]*)?$/);
    return m ? m[1] : null;
  }

  const listingId = extractListingId();
  if (!listingId) return;

  // --- Data extraction ---

  function getText(selectors) {
    for (const sel of selectors) {
      const el = document.querySelector(sel);
      if (el && el.textContent.trim()) return el.textContent.trim();
    }
    return "";
  }

  function getNumber(selectors) {
    const t = getText(selectors);
    const n = parseFloat(t.replace(/[^0-9.]/g, ""));
    return isNaN(n) ? 0 : n;
  }

  // Try JSON-LD first (most reliable when present)
  function extractFromJsonLd() {
    const scripts = document.querySelectorAll('script[type="application/ld+json"]');
    for (const s of scripts) {
      try {
        const d = JSON.parse(s.textContent);
        const obj = Array.isArray(d) ? d[0] : d;
        if (obj["@type"] === "Residence" || obj["@type"] === "RealEstateListing" || obj.name) {
          return obj;
        }
      } catch (_) {}
    }
    return null;
  }

  // Try window.__pg_data__ or similar globals PropertyGuru may use
  function extractFromGlobal() {
    for (const key of ["__pg_data__", "__INITIAL_STATE__", "__NEXT_DATA__", "pgData"]) {
      try {
        const val = window[key];
        if (val) return val;
      } catch (_) {}
    }
    return null;
  }

  function extractImages() {
    const urls = new Set();
    // Slider images
    document.querySelectorAll(
      'img[src*="listing"], img[data-src*="listing"], .listing-carousel img, [class*="gallery"] img, [class*="photo"] img'
    ).forEach((img) => {
      const src = img.getAttribute("src") || img.getAttribute("data-src") || "";
      if (src && !src.includes("placeholder") && src.startsWith("http")) urls.add(src.split("?")[0]);
    });
    // Also look for background-image in style attributes
    document.querySelectorAll('[style*="background-image"]').forEach((el) => {
      const m = el.style.backgroundImage.match(/url\(["']?(https?[^"')]+)["']?\)/);
      if (m) urls.add(m[1].split("?")[0]);
    });
    return Array.from(urls).slice(0, 50);
  }

  function extractPrice() {
    // PropertyGuru shows price as "S$1,250,000" or "$1,250,000"
    const selectors = [
      '[class*="price"]:not([class*="psf"])',
      '[data-testid*="price"]',
      '.listing-price',
      'h2[class*="price"]',
    ];
    for (const sel of selectors) {
      const el = document.querySelector(sel);
      if (el) {
        const t = el.textContent.replace(/[^0-9]/g, "");
        const n = parseInt(t, 10);
        if (n > 100000) return n; // sanity check: SGD price > 100k
      }
    }
    return 0;
  }

  function extractField(patterns) {
    // Look for a dt/dd or label/value pair matching one of the pattern labels
    const pairs = [
      ...document.querySelectorAll("li.listing-property, .listing-attributes li, [class*='attribute'] li"),
    ];
    for (const li of pairs) {
      const text = li.textContent;
      for (const [label, extract] of patterns) {
        if (text.toLowerCase().includes(label.toLowerCase())) {
          const val = extract ? extract(text) : text;
          if (val) return val;
        }
      }
    }
    return "";
  }

  function extractBedrooms() {
    const n = getNumber([
      '[class*="bed"]:not([class*="bath"])',
      '[data-testid*="bedroom"]',
      '.bed-count',
    ]);
    if (n > 0) return n;
    const t = extractField([["bedroom", (t) => t.match(/(\d+)/)?.[1]]]);
    return t ? parseInt(t, 10) : 0;
  }

  function extractBathrooms() {
    const n = getNumber(['[class*="bath"]', '[data-testid*="bathroom"]', ".bath-count"]);
    if (n > 0) return n;
    const t = extractField([["bathroom", (t) => t.match(/(\d+)/)?.[1]]]);
    return t ? parseInt(t, 10) : 0;
  }

  function extractFloorArea() {
    // Look for sqft or sqm values
    const allText = document.body.innerText;
    const m = allText.match(/(\d[\d,]*(?:\.\d+)?)\s*(?:sq\.?\s*ft|sqft)/i);
    if (m) return parseFloat(m[1].replace(/,/g, ""));
    return 0;
  }

  function extractDistrict() {
    // PropertyGuru shows district as "D09" or "District 9"
    const allText = document.body.innerText;
    const m = allText.match(/\bD(\d{1,2})\b/);
    if (m) return `D${m[1].padStart(2, "0")}`;
    return "";
  }

  function buildPayload() {
    const jsonLd = extractFromJsonLd();

    const title = getText([
      "h1",
      '[class*="listing-title"]',
      '[data-testid*="title"]',
      ".property-title",
    ]) || (jsonLd && jsonLd.name) || document.title;

    const price = extractPrice();

    const propertyType = getText([
      '[class*="property-type"]',
      '[data-testid*="property-type"]',
    ]) || "condo";

    const projectName = getText([
      '[class*="project-name"]',
      '[class*="building-name"]',
      '[data-testid*="project"]',
    ]);

    const addressText = getText([
      '[class*="address"]',
      '[itemprop="streetAddress"]',
      '[data-testid*="address"]',
    ]) || (jsonLd && jsonLd.address && jsonLd.address.streetAddress) || "";

    const agentName = getText([
      '[class*="agent-name"]',
      '[data-testid*="agent-name"]',
      '.agent-name',
    ]);

    const agencyName = getText([
      '[class*="agency-name"]',
      '[data-testid*="agency"]',
      '.agency-name',
    ]);

    const description = getText([
      '[class*="description"]',
      '[itemprop="description"]',
      '[data-testid*="description"]',
      '.listing-description',
    ]) || (jsonLd && jsonLd.description) || "";

    const floorLevelText = extractField([
      ["floor", (t) => t.replace(/floor/i, "").trim()],
      ["storey", (t) => t.replace(/storey/i, "").trim()],
      ["level", (t) => t.replace(/level/i, "").trim()],
    ]) || getText(['[class*="floor-level"]', '[data-testid*="floor"]']);

    return {
      source: "propertyguru",
      listing_url: location.href,
      listing_id: listingId,
      captured_at: new Date().toISOString(),
      title: title.slice(0, 500),
      asking_price: price,
      property_type: propertyType.toLowerCase(),
      project_name: projectName.slice(0, 255),
      address_text: addressText.slice(0, 500),
      district: extractDistrict(),
      bedrooms: extractBedrooms(),
      bathrooms: extractBathrooms(),
      floor_area: extractFloorArea(),
      floor_level_text: floorLevelText.slice(0, 100),
      agent_name: agentName.slice(0, 255),
      agency_name: agencyName.slice(0, 255),
      description_text: description.slice(0, 10000),
      image_urls: extractImages(),
    };
  }

  // --- Overlay ---

  function formatPrice(n) {
    if (!n) return "—";
    return "S$" + n.toLocaleString();
  }

  function formatPsf(price, area) {
    if (!price || !area || area <= 0) return "";
    return " (S$" + Math.round(price / area).toLocaleString() + "/sqft)";
  }

  function formatDate(iso) {
    if (!iso) return "—";
    return new Date(iso).toLocaleDateString("en-SG", { day: "numeric", month: "short", year: "numeric" });
  }

  function formatPct(pct) {
    if (pct === undefined || pct === null) return "";
    const sign = pct > 0 ? "+" : "";
    return `${sign}${pct.toFixed(1)}%`;
  }

  function overlayColor(status) {
    switch (status) {
      case "new_listing": return "#2a7d4f";
      case "seen_before": return "#2563eb";
      case "likely_relisted":
      case "almost_certain_same_unit": return "#d97706";
      case "possible_duplicate": return "#7c3aed";
      default: return "#555";
    }
  }

  function statusLabel(status) {
    switch (status) {
      case "new_listing": return "New to database";
      case "seen_before": return "Seen before";
      case "likely_relisted": return "Likely relisted";
      case "almost_certain_same_unit": return "Almost certainly relisted";
      case "possible_duplicate": return "Possible duplicate";
      default: return status;
    }
  }

  function buildOverlayHTML(data, payload) {
    const d = data;
    const area = payload.floor_area;
    const priceChg = d.price_change
      ? `${d.price_change > 0 ? "+" : ""}S$${Math.abs(d.price_change).toLocaleString()} / ${formatPct(d.price_change_pct)}`
      : null;

    const visitRows = (d.client_visit_counts || [])
      .map((c) => `<div class="ut-visit-row"><span>${c.display_name}</span><span>${c.visit_count}×</span></div>`)
      .join("");

    const reasonsHTML = d.match_reasons && d.match_reasons.length
      ? `<div class="ut-section-label">Match reasons</div><div class="ut-reasons">${d.match_reasons.join(" · ")}</div>`
      : "";

    return `
<div id="ut-overlay" style="border-left: 4px solid ${overlayColor(d.status)}">
  <div class="ut-header">
    <span class="ut-brand">UnitTrace</span>
    <button id="ut-close">×</button>
  </div>
  <div class="ut-status" style="color:${overlayColor(d.status)}">${statusLabel(d.status)}</div>
  ${d.match_confidence && d.match_confidence < 100 ? `<div class="ut-confidence">Confidence: ${d.match_confidence}%</div>` : ""}

  <div class="ut-price">
    ${formatPrice(d.current_price)}${formatPsf(d.current_price, area)}
    ${priceChg ? `<div class="ut-price-chg ${d.price_change < 0 ? "down" : "up"}">${priceChg}</div>` : ""}
  </div>

  ${d.first_seen_at ? `
  <div class="ut-section-label">History</div>
  <div class="ut-rows">
    <div class="ut-row"><span>First seen</span><span>${formatDate(d.first_seen_at)} by ${d.first_seen_by || "?"}</span></div>
    <div class="ut-row"><span>Last seen</span><span>${formatDate(d.last_seen_at)} by ${d.last_seen_by || "?"}</span></div>
    <div class="ut-row"><span>Last visited</span><span>${formatDate(d.last_visited_at)} by ${d.last_visited_by || "?"}</span></div>
    <div class="ut-row"><span>Total visits</span><span>${d.visit_count}</span></div>
    ${d.snapshot_count > 1 ? `<div class="ut-row"><span>Versions</span><span>${d.snapshot_count}</span></div>` : ""}
    ${d.possible_relist_count > 0 ? `<div class="ut-row"><span>Relists</span><span>${d.possible_relist_count}</span></div>` : ""}
  </div>` : ""}

  ${visitRows ? `<div class="ut-section-label">Views by person</div><div class="ut-visits">${visitRows}</div>` : ""}

  ${reasonsHTML}

  <div class="ut-note-section">
    <div class="ut-section-label">Add note</div>
    <textarea id="ut-note-input" placeholder="e.g. Agent said price is negotiable…" rows="2"></textarea>
    <button id="ut-note-save">Save note</button>
    <div id="ut-note-status"></div>
  </div>
</div>`;
  }

  function injectStyles() {
    if (document.getElementById("ut-styles")) return;
    const style = document.createElement("style");
    style.id = "ut-styles";
    style.textContent = `
#ut-overlay {
  position: fixed;
  top: 80px;
  right: 16px;
  width: 280px;
  background: #fff;
  border-radius: 10px;
  box-shadow: 0 4px 24px rgba(0,0,0,0.18);
  padding: 14px 16px;
  z-index: 2147483647;
  font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", sans-serif;
  font-size: 13px;
  color: #1a1a1a;
  max-height: 90vh;
  overflow-y: auto;
}
.ut-header { display: flex; justify-content: space-between; align-items: center; margin-bottom: 6px; }
.ut-brand { font-weight: 700; font-size: 14px; letter-spacing: 0.3px; color: #111; }
#ut-close { background: none; border: none; font-size: 20px; cursor: pointer; color: #888; line-height: 1; padding: 0 2px; }
#ut-close:hover { color: #333; }
.ut-status { font-weight: 600; font-size: 14px; margin-bottom: 4px; }
.ut-confidence { font-size: 12px; color: #888; margin-bottom: 8px; }
.ut-price { font-size: 17px; font-weight: 700; color: #111; margin: 10px 0 6px; }
.ut-price-chg { font-size: 13px; font-weight: 500; margin-top: 2px; }
.ut-price-chg.down { color: #2a7d4f; }
.ut-price-chg.up { color: #c0392b; }
.ut-section-label { font-size: 11px; font-weight: 600; text-transform: uppercase; letter-spacing: 0.5px; color: #aaa; margin: 12px 0 5px; }
.ut-rows { display: flex; flex-direction: column; gap: 3px; }
.ut-row { display: flex; justify-content: space-between; gap: 8px; }
.ut-row span:first-child { color: #666; }
.ut-row span:last-child { font-weight: 500; text-align: right; }
.ut-visits { display: flex; flex-direction: column; gap: 3px; }
.ut-visit-row { display: flex; justify-content: space-between; }
.ut-reasons { font-size: 12px; color: #666; line-height: 1.5; }
.ut-note-section { margin-top: 14px; }
#ut-note-input { width: 100%; border: 1px solid #ddd; border-radius: 6px; padding: 6px 8px; font-size: 13px; resize: vertical; font-family: inherit; }
#ut-note-save { margin-top: 6px; background: #4f7dff; color: white; border: none; padding: 6px 14px; border-radius: 6px; font-size: 13px; cursor: pointer; }
#ut-note-save:hover { background: #3a68e8; }
#ut-note-status { font-size: 12px; color: #2a7d4f; margin-top: 4px; min-height: 16px; }
    `;
    document.head.appendChild(style);
  }

  function renderOverlay(data, payload) {
    document.getElementById("ut-overlay")?.remove();
    injectStyles();
    const div = document.createElement("div");
    div.innerHTML = buildOverlayHTML(data, payload);
    document.body.appendChild(div.firstElementChild);

    document.getElementById("ut-close").addEventListener("click", () => {
      document.getElementById("ut-overlay")?.remove();
    });

    const noteInput = document.getElementById("ut-note-input");
    const noteSave = document.getElementById("ut-note-save");
    const noteStatus = document.getElementById("ut-note-status");

    noteSave.addEventListener("click", () => {
      const note = noteInput.value.trim();
      if (!note) return;
      noteSave.disabled = true;
      chrome.runtime.sendMessage(
        { type: "ADD_NOTE", tracked_unit_id: data.tracked_unit_id, note },
        (res) => {
          noteSave.disabled = false;
          if (res && res.ok) {
            noteInput.value = "";
            noteStatus.textContent = "Note saved.";
            setTimeout(() => { noteStatus.textContent = ""; }, 2000);
          } else {
            noteStatus.textContent = "Error saving note.";
          }
        }
      );
    });
  }

  function renderError(msg) {
    document.getElementById("ut-overlay")?.remove();
    injectStyles();
    const div = document.createElement("div");
    div.innerHTML = `<div id="ut-overlay" style="border-left:4px solid #ccc">
      <div class="ut-header"><span class="ut-brand">UnitTrace</span><button id="ut-close">×</button></div>
      <div style="color:#888;font-size:12px;margin-top:6px">${msg}</div>
    </div>`;
    document.body.appendChild(div.firstElementChild);
    document.getElementById("ut-close").addEventListener("click", () => {
      document.getElementById("ut-overlay")?.remove();
    });
  }

  // --- Main ---

  async function run() {
    const payload = buildPayload();

    chrome.runtime.sendMessage({ type: "SUBMIT_LISTING", data: payload }, (res) => {
      if (chrome.runtime.lastError) {
        renderError("Could not connect to UnitTrace.");
        return;
      }
      if (!res || !res.ok) {
        renderError(`Backend error: ${res?.error || "unknown"}`);
        return;
      }
      renderOverlay(res.data, payload);
    });
  }

  // Small delay to let the page fully render before scraping
  setTimeout(run, 1500);
})();
