"use client";

import { signIn } from "next-auth/react";
import { Button } from "@/components/ui/button";
import { Card } from "@/components/ui/card";

export default function LoginPage() {
  return (
    <div className="flex items-center justify-center min-h-screen bg-gray-100">
      <Card className="w-96 p-8 shadow-lg">
        <div className="space-y-6">
          <div className="text-center">
            <h1 className="text-3xl font-bold text-gray-900">Pred</h1>
            <p className="text-gray-600 mt-2">
              Predictive Maintenance System
            </p>
          </div>

          <Button
            onClick={() =>
              signIn("keycloak", {
                redirect: true,
                callbackUrl: "/dashboard",
              })
            }
            className="w-full py-6 text-lg font-semibold"
          >
            Login with Keycloak
          </Button>

          <p className="text-center text-sm text-gray-500">
            Contact your administrator for account access
          </p>
        </div>
      </Card>
    </div>
  );
}
