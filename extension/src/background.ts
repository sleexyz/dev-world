console.log("background.ts");

// Install PAC script on install
chrome.runtime.onInstalled.addListener(() => {
  console.log("onInstalled...");
  chrome.proxy.settings.set(
    {
      value: {
        mode: "pac_script",
        pacScript: {
          data: `
            function FindProxyForURL(url, host) {
                if (host == 'dev' || host == 'd') {
                    return "HTTPS localhost:12345";
                }
                return "DIRECT";
            }
            `,
        },
      },
    },
    () => {
      console.log("set proxy");
    }
  );
});
