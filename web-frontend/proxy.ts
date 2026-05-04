import { type NextRequest, NextResponse } from "next/server";
import { getToken } from "next-auth/jwt";

export async function proxy(request: NextRequest) {
	const token = await getToken({
		req: request,
		secret: process.env.NEXTAUTH_SECRET,
	});
	const { pathname } = request.nextUrl;

	// Redirect authenticated users away from the login page
	if (pathname === "/login" && token) {
		return NextResponse.redirect(new URL("/dashboard", request.url));
	}

	// Redirect unauthenticated users to login for all protected routes
	if (pathname !== "/login" && !token) {
		const loginUrl = new URL("/login", request.url);
		loginUrl.searchParams.set("callbackUrl", pathname);
		return NextResponse.redirect(loginUrl);
	}

	return NextResponse.next();
}

export const config = {
	matcher: [
		"/dashboard/:path*",
		"/motors/:path*",
		"/alerts/:path*",
		"/assets/:path*",
		"/predictions/:path*",
		"/login",
	],
};
