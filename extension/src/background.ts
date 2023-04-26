const PACMAN_EXTENSION_ID = "jbclhgpgaijegjnfhbpidgaooihpcphd";

chrome.runtime.onInstalled.addListener(async () => {
  //Install PAC script on install by messaging the pacman extension
  await Promise.all([
    chrome.runtime.sendMessage(PACMAN_EXTENSION_ID, {
      setProxyRequest: {
        protocol: "http",
        host: "dev",
        type: "PROXY",
        destination: "localhost:12345",
      },
    }),
    chrome.runtime.sendMessage(PACMAN_EXTENSION_ID, {
      setProxyRequest: {
        protocol: "https",
        host: "dev",
        type: "HTTPS",
        destination: "localhost:12345",
      },
    }),
    chrome.runtime.sendMessage(PACMAN_EXTENSION_ID, {
      setProxyRequest: {
        protocol: "http",
        host: "d",
        type: "PROXY",
        destination: "localhost:12345",
      },
    }),
    chrome.runtime.sendMessage(PACMAN_EXTENSION_ID, {
      setProxyRequest: {
        protocol: "https",
        host: "d",
        type: "HTTPS",
        destination: "localhost:12345",
      },
    }),
  ]);
});

loop({ maxRunsPerSec: 1 }, async () => {
  // Listen to SSE events from /api/listen-open-file
  const eventSource = new EventSource(
    "https://localhost:12345/api/listen-open-file"
  );

  // Listen to the "open" event
  eventSource.addEventListener("open", (event: Event) => {
    console.log("Connection opened", event);
  });
  // Listen to the "message" event
  eventSource.addEventListener("message", (event: MessageEvent) => {
    console.log("Message received:", event.data);
    openFile(JSON.parse(event.data));
  });

  const cbs: { resolve: () => void; reject: (reason?: unknown) => void }[] = [];
  eventSource.addEventListener("error", (event: Event) => {
    console.log("Error occurred:", event);
    for (const cb of cbs) {
      cb.reject();
    }
  });
  await new Promise<void>((resolve, reject) => {
    cbs.push({ resolve, reject });
  });
});

function openFile({ file }: { file: string; line: number; column: number }) {
  console.log("Opening file", file);
  openTabForWorkspace(file);
}
// Iterate through all tabs and focus on the first matching tab
function openTabForWorkspace(file: string) {
  chrome.tabs.query({}, (tabs) => {
    for (const tab of tabs) {
      if (!tab.url) {
        continue;
      }
      if (!tab.id) {
        continue;
      }
      const tabAlias = /d\/(.*)/.exec(tab.url)?.[1];
      if (!tabAlias) {
        continue;
      }
      const tabPath = "/Users/slee2/" + tabAlias;
      if (file.startsWith(tabPath)) {
        console.log("Found matching tab for file", file, tab);
        chrome.tabs.update(tab.id, { active: true });
        return;
      }
    }
  });
}

async function loop(
  options: { maxRunsPerSec: number },
  fn: () => Promise<void>
) {
  let runs = 0;
  let lastSampledTime = Date.now();
  for (;;) {
    await fn();
    runs += 1;
    // Break if we've run 3 * n times within 3 seconds, aka avg of n times per second:
    if (runs >= 3) {
      if (Date.now() - lastSampledTime < 3 * 1000 * options.maxRunsPerSec) {
        break;
      }
      lastSampledTime = Date.now();
      runs = 0;
    }
  }
}
