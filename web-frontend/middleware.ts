import { NextRequest, NextResponse } from "next/server";

export async function middleware(request: NextRequest) {
  // For now, allow all requests to pass through
  // TODO: Implement proper middleware authentication
  return NextResponse.next();
}

export const config = {
  matcher: ["/dashboard/:path*", "/login"],
};
