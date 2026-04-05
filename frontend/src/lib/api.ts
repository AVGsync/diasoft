import type { Session } from "./types";

const GATEWAY_BASE = "/api/v1";
const VERIFY_BASE = "/verify-api/api/v1";
const SESSION_KEY = "diasoft.frontend.session";

type RequestBody = BodyInit | Record<string, unknown> | undefined;

interface RequestOptions {
  method?: string;
  token?: string;
  body?: RequestBody;
  headers?: Record<string, string>;
  responseType?: "json" | "blob";
  verifyApi?: boolean;
}

export interface DownloadProgress {
  stage: "preparing" | "downloading" | "completed";
  loadedBytes: number;
  totalBytes?: number;
}

export async function apiRequest<T>(path: string, options: RequestOptions = {}): Promise<T> {
  const {
    method = "GET",
    token,
    body,
    headers = {},
    responseType = "json",
    verifyApi = false,
  } = options;

  const { base, init } = buildRequestInit({ method, token, body, headers, verifyApi });
  const response = await fetch(`${base}${path}`, init);

  if (!response.ok) {
    throw new Error(await readErrorMessage(response));
  }

  if (responseType === "blob") {
    return (await response.blob()) as T;
  }

  if (response.status === 204) {
    return undefined as T;
  }

  return (await response.json()) as T;
}

export async function downloadFileWithProgress(
  path: string,
  fileName: string,
  options: RequestOptions = {},
  onProgress?: (progress: DownloadProgress) => void,
) {
  const { method = "GET", token, body, headers = {}, verifyApi = false } = options;
  const { base, init } = buildRequestInit({ method, token, body, headers, verifyApi });

  onProgress?.({ stage: "preparing", loadedBytes: 0 });

  const response = await fetch(`${base}${path}`, init);
  if (!response.ok) {
    throw new Error(await readErrorMessage(response));
  }

  const totalHeader = response.headers.get("Content-Length");
  const totalBytes = totalHeader ? Number(totalHeader) : undefined;

  if (!response.body) {
    const blob = await response.blob();
    onProgress?.({
      stage: "downloading",
      loadedBytes: blob.size,
      totalBytes: totalBytes ?? blob.size,
    });
    downloadBlob(blob, fileName);
    onProgress?.({
      stage: "completed",
      loadedBytes: blob.size,
      totalBytes: totalBytes ?? blob.size,
    });
    return;
  }

  const reader = response.body.getReader();
  const chunks: ArrayBuffer[] = [];
  let loadedBytes = 0;

  onProgress?.({ stage: "downloading", loadedBytes: 0, totalBytes });

  while (true) {
    const { done, value } = await reader.read();
    if (done) {
      break;
    }

    if (!value) {
      continue;
    }

    chunks.push(value.slice().buffer);
    loadedBytes += value.byteLength;
    onProgress?.({ stage: "downloading", loadedBytes, totalBytes });
  }

  const blob = new Blob(chunks, {
    type: response.headers.get("Content-Type") || "application/octet-stream",
  });
  downloadBlob(blob, fileName);
  onProgress?.({
    stage: "completed",
    loadedBytes,
    totalBytes: totalBytes ?? loadedBytes,
  });
}

function buildRequestInit(options: {
  method: string;
  token?: string;
  body?: RequestBody;
  headers?: Record<string, string>;
  verifyApi?: boolean;
}) {
  const { method, token, body, headers = {}, verifyApi = false } = options;
  const requestHeaders: Record<string, string> = { ...headers };
  const init: RequestInit = { method, headers: requestHeaders, cache: "no-store" };
  const isFormData = typeof FormData !== "undefined" && body instanceof FormData;

  if (token) {
    requestHeaders.Authorization = `Bearer ${token}`;
  }

  if (body !== undefined) {
    if (isFormData) {
      init.body = body as FormData;
    } else if (typeof body === "string" || body instanceof Blob) {
      init.body = body as BodyInit;
    } else {
      requestHeaders["Content-Type"] = "application/json";
      init.body = JSON.stringify(body);
    }
  }

  const base = verifyApi ? VERIFY_BASE : GATEWAY_BASE;
  return { base, init };
}

async function readErrorMessage(response: Response) {
  let message = "Request failed";

  try {
    const payload = (await response.json()) as { error?: string; message?: string };
    message = payload.error ?? payload.message ?? message;
  } catch {
    message = response.statusText || message;
  }

  return message;
}

export function loadSession(): Session | null {
  const raw = window.localStorage.getItem(SESSION_KEY);
  if (!raw) {
    return null;
  }

  try {
    return JSON.parse(raw) as Session;
  } catch {
    window.localStorage.removeItem(SESSION_KEY);
    return null;
  }
}

export function saveSession(session: Session) {
  window.localStorage.setItem(SESSION_KEY, JSON.stringify(session));
}

export function clearSession() {
  window.localStorage.removeItem(SESSION_KEY);
}

export function downloadBlob(blob: Blob, fileName: string) {
  const url = window.URL.createObjectURL(blob);
  const anchor = document.createElement("a");
  anchor.href = url;
  anchor.download = fileName;
  anchor.style.display = "none";
  document.body.appendChild(anchor);
  anchor.click();
  anchor.remove();
  window.setTimeout(() => {
    window.URL.revokeObjectURL(url);
  }, 1000);
}
