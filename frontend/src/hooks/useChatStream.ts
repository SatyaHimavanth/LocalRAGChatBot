// frontend/src/hooks/useChatStream.ts
import { useEffect, useState } from "react";
import { Events } from "@wailsio/runtime";
import { SendMessage } from "../../bindings/changeme/internal/app/chatservice";

export function useChatStream(sessionId: number) {
	const [text, setText] = useState("");
	const [done, setDone] = useState(false);

	useEffect(() => {
		const offToken = Events.On("chat:token", (e: any) => {
			if (e.data.sessionId === sessionId) setText((t) => t + e.data.token);
		});
		const offDone = Events.On("chat:done", () => setDone(true));
		return () => {
			offToken();
			offDone();
		};
	}, [sessionId]);

	const send = (collectionId: number, prompt: string) => {
		setText("");
		setDone(false);
		SendMessage(sessionId, collectionId, prompt);
	};

	return { text, done, send };
}
