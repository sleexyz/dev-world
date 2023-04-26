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
