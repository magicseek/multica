"use client";

import { use } from "react";
import { ChatSessionPage } from "@multica/views/chat";

export default function Page({
  params,
}: {
  params: Promise<{ id: string }>;
}) {
  const { id } = use(params);
  return <ChatSessionPage sessionId={id} />;
}
