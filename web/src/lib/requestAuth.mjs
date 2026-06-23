export function shouldAttachAuthHeader(requestUrl, apiBaseUrl, currentOrigin) {
  if (!requestUrl) {
    return false;
  }

  if (!/^https?:\/\//i.test(requestUrl)) {
    return true;
  }

  try {
    const request = new URL(requestUrl);
    if (currentOrigin && request.origin === new URL(currentOrigin).origin) {
      return true;
    }
    if (apiBaseUrl && /^https?:\/\//i.test(apiBaseUrl)) {
      return request.origin === new URL(apiBaseUrl).origin;
    }
  } catch {
    return false;
  }

  return false;
}
