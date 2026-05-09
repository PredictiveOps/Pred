"use client";

import { AssetDetail } from "@/components/ui/asset-detail";

export default function MotorsPage() {
	return <AssetDetail onBack={() => window.history.back()} />;
}
