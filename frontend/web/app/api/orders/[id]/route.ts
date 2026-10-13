import { NextResponse } from "next/server";
import { ApiError } from "@/lib/api/server";
import { getAccessToken } from "@/lib/auth/session";
import { getMyOrder } from "@/lib/data/orders";

/** Order status for the confirmation page's polling; the access token stays in its httpOnly cookie. */
export async function GET(_req: Request, ctx: { params: Promise<{ id: string }> }) {
  const token = await getAccessToken();
  if (!token) return NextResponse.json({ message: "Unauthorized" }, { status: 401 });
  const { id } = await ctx.params;
  try {
    const order = await getMyOrder(token, id);
    if (!order) return NextResponse.json({ message: "Order not found" }, { status: 404 });
    return NextResponse.json(order, { headers: { "Cache-Control": "no-store" } });
  } catch (e) {
    const status = e instanceof ApiError ? e.status : 502;
    return NextResponse.json({ message: "Could not load the order" }, { status });
  }
}
