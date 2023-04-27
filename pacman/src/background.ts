interface ProxyEntry {
  protocol: string;
  host: string;
  type: string;
  destination: string;
  extensionId: string;
}
interface SetProxyRequest {
  protocol: string;
  host: string;
  type: string;
  destination: string;
}

export type SetProxyResponse = boolean;

class ProxyManager {
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

  addEntry(entry: ProxyEntry) {
    const key = entry.protocol + "://" + entry.host;
    this.entries[key] = entry;
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
        console.log("Set PAC script: ", pacScript);
      }
    );
  }

  save() {
    chrome.storage.local.set({ entries: this.entries });
  }

  static async load(): Promise<ProxyManager> {
    const proxyManager = await new Promise<ProxyManager>((resolve) => {
      chrome.storage.local.get("entries", (result) => {
        const instance = new ProxyManager();
        if (result.entries) {
          for (const entry of Object.values(
            result.entries as Record<string, ProxyEntry>
          )) {
            instance.addEntry(entry);
          }
        }
        resolve(instance);
      });
    });
    proxyManager.applySettings();
    return proxyManager;
  }

  async handleSetProxyRequest(
    { protocol, host, type, destination }: SetProxyRequest,
    extensionId: string
  ): Promise<SetProxyResponse> {
    try {
      this.addEntry({
        protocol,
        host,
        type,
        destination,
        extensionId,
      });
      this.save();
      this.applySettings();
      return true;
    } catch (e) {
      return false;
    }
  }

  listen() {
    chrome.runtime.onMessageExternal.addListener(
      async (request, sender, sendResponse) => {
        if (!sender.id) {
          return;
        }
        if (request.setProxyRequest) {
          const setProxyResponse = await this.handleSetProxyRequest(
            request.setProxyRequest as SetProxyRequest,
            sender.id
          );
          sendResponse({ setProxyResponse });
        }
      }
    );
  }
}

(async () => {
  const proxyManager = await ProxyManager.load();
  proxyManager.listen();
})();
