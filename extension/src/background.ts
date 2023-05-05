const PACMAN_EXTENSION_ID = "jeifocpihhhiljnipffnppembibdlhai";

class Extension {
  async registerExtension() {
    await Promise.all([
      chrome.runtime.sendMessage(PACMAN_EXTENSION_ID, {
        setProxyRequest: {
          host: "dev",
          type: "PROXY",
          destination: "localhost:12345",
        },
      }),
      chrome.runtime.sendMessage(PACMAN_EXTENSION_ID, {
        setProxyRequest: {
          host: "d",
          type: "PROXY",
          destination: "localhost:12345",
        },
      }),
    ]);
  }

  listen() {
    runForever({ maxRunsPerSec: 3 }, async () => {
      const eventSource = new EventSource(
        "https://localhost:12345/api/listen-open-file"
      );
      eventSource.addEventListener("open", onOpen) 
      eventSource.addEventListener("message", onMessage);

      function onOpen (event: Event)  {
        console.log("Connection opened", event);
      }
      function onMessage (event: MessageEvent) {
        console.log("Message received:", event.data);
        openFile(JSON.parse(event.data));
      }

      // Wait for error
      const cbs: { resolve: () => void; reject: (reason?: unknown) => void }[] =
        [];
      eventSource.addEventListener("error", onError);
      function onError(event: Event) {
        console.log("Error occurred:", event);
        for (const cb of cbs) {
          cb.reject();
        }
      }
      await new Promise<void>((resolve, reject) => {
        cbs.push({ resolve, reject });
      });

      // Cleanup
      eventSource.removeEventListener("error", onError);
      eventSource.removeEventListener("open", onOpen);
      eventSource.removeEventListener("message", onMessage);
      eventSource.close();
    });
  }
}

(async () => {
  const extension = new Extension();
  await extension.registerExtension();
  console.log("Registered extension");
})();

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

async function runForever(
  options: { maxRunsPerSec: number },
  fn: () => Promise<void>
) {
  for (;;) {
    const start = Date.now();
    await fn();
    const timeElapsed = Date.now() - start;
    if (timeElapsed < 1000 / options.maxRunsPerSec) {
      await new Promise((resolve) =>
        setTimeout(resolve, 1000 / options.maxRunsPerSec - timeElapsed)
      );
    }
  }
}
