const PACMAN_EXTENSION_ID = "jbclhgpgaijegjnfhbpidgaooihpcphd";

chrome.runtime.onInstalled.addListener(async () => {
  //Install PAC script on install by messaging the pacman extension
  const response = await chrome.runtime.sendMessage(PACMAN_EXTENSION_ID, {
    setProxyRequest: {
      host: "dev",
      destination: "localhost:12345",
      type: "HTTPS",
    },
  });
  if (response.setProxyResponse) {
    console.log("Set proxy");
  }
});
