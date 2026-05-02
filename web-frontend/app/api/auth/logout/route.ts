import { signOut } from "next-auth/react";

// This endpoint handles logout and redirects to Keycloak logout
export async function POST() {
  // SignOut will clear the session
  // You can also redirect to Keycloak's logout endpoint if needed
  const keycloakLogoutUrl = `${process.env.KEYCLOAK_URL}/realms/${process.env.KEYCLOAK_REALM}/protocol/openid-connect/logout?redirect_uri=${process.env.NEXTAUTH_URL}`;

  return Response.redirect(keycloakLogoutUrl);
}
