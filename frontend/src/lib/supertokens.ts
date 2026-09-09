import SuperTokens from "supertokens-web-js";
import Session from "supertokens-web-js/recipe/session";
import EmailPassword from "supertokens-web-js/recipe/emailpassword";
import EmailVerification from "supertokens-web-js/recipe/emailverification";

const API_URL = (process.env.NEXT_PUBLIC_API_URL || "http://localhost:8080")
  .replace(/\/api\/v1\/?$/, "")
  .replace(/\/+$/, "");

let initialized = false;

export function initSuperTokens() {
  if (typeof window === "undefined" || initialized) {
    return;
  }

  SuperTokens.init({
    appInfo: {
      appName: "Lynk",
      apiDomain: API_URL,
      apiBasePath: "/api/v1/auth",
    },
    recipeList: [
      Session.init(),
      EmailPassword.init(),
      EmailVerification.init(),
    ],
  });

  initialized = true;
}

export { Session, EmailPassword, EmailVerification };
