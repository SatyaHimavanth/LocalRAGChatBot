// frontend/src/components/ChatWindow.tsx (minimal)
import { useState } from "react";
import { useChatStream } from "../hooks/useChatStream";

export function ChatWindow({ sessionId, collectionId }: { sessionId: number; collectionId: number }) {
  const { text, send } = useChatStream(sessionId);
  const [prompt, setPrompt] = useState("");

  return (
    <div>
      <div className="messages">{text}</div>
      <input value={prompt} onChange={(e) => setPrompt(e.target.value)} />
      <button onClick={() => send(collectionId, prompt)}>Send</button>
    </div>
  );
}
