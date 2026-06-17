const nameInput = document.getElementById("displayName");
const saveBtn = document.getElementById("saveBtn");
const statusEl = document.getElementById("status");
const clientIdEl = document.getElementById("clientId");

chrome.storage.local.get(["client_id", "display_name"], (result) => {
  if (result.client_id) clientIdEl.textContent = result.client_id;
  if (result.display_name) nameInput.value = result.display_name;
});

saveBtn.addEventListener("click", () => {
  const name = nameInput.value.trim();
  if (!name) {
    statusEl.textContent = "Display name cannot be empty.";
    statusEl.className = "status";
    return;
  }
  chrome.storage.local.set({ display_name: name }, () => {
    statusEl.textContent = "Saved.";
    statusEl.className = "status ok";
    setTimeout(() => { statusEl.textContent = ""; }, 2000);
  });
});
