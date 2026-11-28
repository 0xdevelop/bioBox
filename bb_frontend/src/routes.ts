export type AppPage = "business" | "manage";

export type AppRoute = {
  page: AppPage;
};

export function normalizePath(pathname: string): string {
  const trimmed = pathname.trim();
  if (trimmed === "") {
    return "/";
  }
  if (trimmed.length > 1 && trimmed.endsWith("/")) {
    return trimmed.replace(/\/+$/, "") || "/";
  }
  return trimmed;
}

export function isAppPath(pathname: string): boolean {
  const path = normalizePath(pathname);
  return path === "/" || path === "/business" || path === "/manage";
}

export function routeFromPath(pathname: string): AppRoute {
  if (normalizePath(pathname) === "/manage") {
    return { page: "manage" };
  }
  return { page: "business" };
}

export function pathForPage(page: AppPage): string {
  return page === "manage" ? "/manage" : "/business";
}
