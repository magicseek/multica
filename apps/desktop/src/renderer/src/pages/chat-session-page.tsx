import { useParams } from "react-router-dom";
import { useQuery } from "@tanstack/react-query";
import { ChatSessionPage as ChatSessionPageView } from "@multica/views/chat";
import { useWorkspaceId } from "@multica/core/hooks";
import { chatSessionOptions } from "@multica/core/chat/queries";
import { useDocumentTitle } from "@/hooks/use-document-title";

export function ChatSessionPage() {
  const { id } = useParams<{ id: string }>();
  const wsId = useWorkspaceId();
  const { data: session } = useQuery(chatSessionOptions(wsId, id ?? ""));

  useDocumentTitle(session?.title || "Chat");

  if (!id) return null;
  return <ChatSessionPageView sessionId={id} />;
}
