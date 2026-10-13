import type { Metadata } from "next";
import { AppointmentSection } from "@/components/home/appointment-section";
import { getI18n } from "@/lib/i18n/server";

export async function generateMetadata({ params }: PageProps<"/[locale]/appointment">): Promise<Metadata> {
  const { d } = await getI18n(params);
  return { title: d.appointment.pageTitle };
}

export default function AppointmentPage() {
  return <AppointmentSection />;
}
