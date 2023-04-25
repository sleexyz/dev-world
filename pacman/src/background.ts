interface ProxyEntry {
  host: string;
  destination: string;
  type: "HTTP" | "HTTPS";
  extensionId: string;
}

class ProxyManager {
  static instance: ProxyManager = new ProxyManager();

  // alias -> ProxyEntry
  constructor(readonly entries: Map<string, ProxyEntry> = new Map()) {}

  private generatePacScript(): string {
    return `
function FindProxyForURL(url, host) {
  const data = ${JSON.stringify(Array.from(this.entries.values()), null, 2)};
  for (const entry of data) {
    if (host == entry.host) {
      return entry.type + ' ' + entry.destination;
    }
  }
  return 'DIRECT';
}
`;
  }

  addEntry(entry: ProxyEntry) {
    this.entries.set(entry.host, entry);
    this.applySettings();
  }

  applySettings() {
    const pacScript = this.generatePacScript();
    chrome.proxy.settings.set(
      {
        value: {
          mode: "pac_script",
          pacScript: {
            data: pacScript,
          },
        },
      },
      () => {
        console.log("Set PAC script: ", pacScript)
      }
    );
  }

  load() {
    chrome.storage.local.get("entries", (result) => {
      if (result.entries) {
        for (const entry of result.entries) {
          this.addEntry(entry);
        }
      }
    });
  }

  save() {
    chrome.storage.local.set({ entries: Array.from(this.entries.values()) });
  }
}

// Install PAC script on install
chrome.runtime.onInstalled.addListener(() => {
  ProxyManager.instance.load();
  ProxyManager.instance.applySettings();
});


interface SetProxyRequest {
  host: string;
  destination: string;
  type: "HTTP" | "HTTPS";
}

type SetProxyResponse = boolean;

chrome.runtime.onMessageExternal.addListener(
  (request, sender, sendResponse) => {
    if (request.setProxyRequest) {
      const { host, destination, type } = request.setProxyRequest as SetProxyRequest;
      const { id: extensionId } = sender;
      if (!extensionId) {
        sendResponse({ setProxyResponse: false });
        return;
      }
      ProxyManager.instance.addEntry({ host, destination, type, extensionId });
      ProxyManager.instance.save();
      sendResponse({ setProxyResponse: true });
    }
  }
);
