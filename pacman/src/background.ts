interface ProxyEntry {
  protocol: string;
  host: string;
  type: string;
  destination: string;
  extensionId: string;
}

class ProxyManager {
  static instance: ProxyManager = new ProxyManager();

  // alias -> ProxyEntry
  constructor(readonly entries: Record<string, ProxyEntry> = {}) {}

  private generatePacScript(): string {
    return `
function FindProxyForURL(url, host) {
  const data = ${JSON.stringify(this.entries, null, 2)};
  const protocol = url.substring(0, url.indexOf(':'));
  let entry = data[protocol + '://' + host];
  if (entry) {
    return entry.type + ' ' + entry.destination;
  }
  return 'DIRECT';
}
`;
  }

  static makeEntryKey(protocol: string, host: string) {
    return `${protocol}://${host}`;
  }
  addEntry(entry: ProxyEntry) {
    const key = ProxyManager.makeEntryKey(entry.protocol, entry.host)
    this.entries[key] =  entry;
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
        for (const entry of Object.values(result.entries as Record<string, ProxyEntry>)) {
          this.addEntry(entry);
        }
      }
    });
  }

  save() {
    chrome.storage.local.set({ entries: this.entries });
  }
}

// Install PAC script on install
chrome.runtime.onInstalled.addListener(() => {
  ProxyManager.instance.load();
  ProxyManager.instance.applySettings();
});


interface SetProxyRequest {
  protocol: string;
  host: string;
  type: string;
  destination: string;
}

export type SetProxyResponse = boolean;

chrome.runtime.onMessageExternal.addListener(
  (request, sender, sendResponse) => {
    if (request.setProxyRequest) {
      const { protocol, host, type, destination } = request.setProxyRequest as SetProxyRequest;
      const { id: extensionId } = sender;
      if (!extensionId) {
        sendResponse({ setProxyResponse: false });
        return;
      }
      ProxyManager.instance.addEntry({ protocol, host, type, destination, extensionId });
      ProxyManager.instance.save();
      sendResponse({ setProxyResponse: true });
    }
  }
);
