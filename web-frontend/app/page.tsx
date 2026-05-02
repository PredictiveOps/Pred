import { getServerSession } from "next-auth/next";
import { authOptions } from "@/lib/auth";
import { redirect } from "next/navigation";
import Link from "next/link";

export default async function Home() {
  const session = await getServerSession(authOptions);

  // If user is logged in, redirect to dashboard
  if (session) {
    redirect("/dashboard");
  }

  // If not logged in, show landing page with login option
  return (
    <div className="flex items-center justify-center min-h-screen bg-gradient-to-br from-blue-50 to-indigo-100">
      <div className="max-w-md mx-auto text-center">
        <h1 className="text-4xl font-bold text-gray-900 mb-4">Pred</h1>
        <p className="text-lg text-gray-600 mb-8">
          Predictive Maintenance System
        </p>
        <p className="text-gray-500 mb-8">
          Manage your equipment and receive intelligent maintenance recommendations
        </p>
        <Link
          href="/login"
          className="inline-block bg-indigo-600 hover:bg-indigo-700 text-white font-semibold py-3 px-8 rounded-lg transition"
        >
          Login
        </Link>
      </div>
    </div>
  );
}
