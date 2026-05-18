import { redirect } from "next/navigation";

export default async function RuntimesRedirectPage({
  params,
}: {
  params: Promise<{ workspaceSlug: string }>;
}) {
  const { workspaceSlug } = await params;
  redirect(`/${encodeURIComponent(workspaceSlug)}/settings?tab=runtimes`);
}
