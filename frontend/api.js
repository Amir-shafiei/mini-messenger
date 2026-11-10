// HTTP and session renewal. Cookies remain HttpOnly; JavaScript never reads tokens.
export async function request(path, options = {}) {
  let response;
  try {
    response = await fetch("/api/v1" + path, {
      credentials: "same-origin",
      ...options,
      headers: { "Content-Type": "application/json", ...options.headers },
      signal: AbortSignal.timeout(15000),
    });
  } catch {
    throw new Error("به سرور نرسیدیم. یه بار دیگه امتحان کن.");
  }
  if (!response.ok) {
    const body = await response.json().catch(() => null);
    const code = body?.error?.code;
    const error = new Error(errorMessage(response.status, code, path));
    error.status = response.status;
    error.code = code;
    throw error;
  }
  return response.status === 204 ? null : response.json();
}

function errorMessage(status, code, path) {
  if (code === "invalid_idempotency_key")
    return "نسخهٔ این صفحه قدیمیه؛ متن رو کپی کن و صفحه رو دوباره باز کن.";
  if (status === 401)
    return path === "/auth/login"
      ? "اسم کاربری یا رمزت درست نیست."
      : "دوباره وارد شو تا ادامه بدیم.";
  if (status === 429) return "یه کم تند رفتیم! یک دقیقه صبر کن.";
  if (status === 403) return "آدرس برنامه درست نیست؛ صفحه رو از لینک اجرای Docker باز کن.";
  if (status === 400) return "اطلاعات این درخواست درست نیست؛ دوباره چکش کن.";
  if (status === 409) return "این ارسال قبلاً با یه متن دیگه ثبت شده.";
  return "این بار نشد. یه بار دیگه امتحان کن.";
}

let renewalInProgress = null;

export function withSessionLock(action) {
  if (!navigator.locks) {
    throw new Error("برای تمدید امن ورود، مرورگرت رو به‌روز کن.");
  }
  return navigator.locks.request("mini-messenger-session", action);
}

export function renewSession() {
  if (renewalInProgress) return renewalInProgress;
  // Shared cookies mean multiple tabs must not rotate the same refresh token.
  renewalInProgress = withSessionLock(async () => {
    try {
      const current = await request("/auth/me");
      if (new Date(current.expires_at) - Date.now() > 90000) return current;
    } catch (error) {
      if (error.status !== 401) throw error;
    }
    // Never blindly retry this POST: the previous rotation may have committed.
    return request("/auth/refresh", { method: "POST" });
  }).finally(() => {
    renewalInProgress = null;
  });
  return renewalInProgress;
}
