// Service worker — manages client_id persistence and acts as message relay.

const BACKEND_URL = "https://unittrace.tet.sg";

async function getIdentity() {
  const result = await chrome.storage.local.get(["client_id", "display_name"]);
  let clientId = result.client_id;
  if (!clientId) {
    clientId = crypto.randomUUID();
    await chrome.storage.local.set({ client_id: clientId });
  }
  return {
    client_id: clientId,
    display_name: result.display_name || "Unknown",
    backend_url: BACKEND_URL,
  };
}

chrome.runtime.onMessage.addListener((msg, sender, sendResponse) => {
  if (msg.type === "GET_IDENTITY") {
    getIdentity().then(sendResponse);
    return true; // async
  }

  if (msg.type === "SUBMIT_LISTING") {
    getIdentity().then(async (identity) => {
      try {
        const payload = { ...msg.data, ...identity };
        const res = await fetch(`${identity.backend_url}/api/v1/listing-views`, {
          method: "POST",
          headers: { "Content-Type": "application/json" },
          body: JSON.stringify(payload),
        });
        const data = await res.json();
        sendResponse({ ok: true, data });
      } catch (err) {
        sendResponse({ ok: false, error: err.message });
      }
    });
    return true;
  }

  if (msg.type === "BATCH_STATUS") {
    getIdentity().then(async (identity) => {
      try {
        const res = await fetch(`${identity.backend_url}/api/v1/listing-status-batch`, {
          method: "POST",
          headers: { "Content-Type": "application/json" },
          body: JSON.stringify({ source: "propertyguru", listing_ids: msg.listing_ids }),
        });
        const data = await res.json();
        sendResponse({ ok: true, data });
      } catch (err) {
        sendResponse({ ok: false, error: err.message });
      }
    });
    return true;
  }

  if (msg.type === "ADD_NOTE") {
    getIdentity().then(async (identity) => {
      try {
        const res = await fetch(
          `${identity.backend_url}/api/v1/tracked-units/${msg.tracked_unit_id}/notes`,
          {
            method: "POST",
            headers: { "Content-Type": "application/json" },
            body: JSON.stringify({
              client_id: identity.client_id,
              display_name: identity.display_name,
              note: msg.note,
            }),
          }
        );
        const data = await res.json();
        sendResponse({ ok: true, data });
      } catch (err) {
        sendResponse({ ok: false, error: err.message });
      }
    });
    return true;
  }
});
