// frontend/src/hooks/useChats.ts
import { useState, useEffect, useCallback } from "react";
import { Events } from "@wailsio/runtime";
import {
	CreateChat,
	GetChats,
	GetChatMessages,
	DeleteChat,
	SendMessage,
} from "../../bindings/changeme/internal/app/chatservice";

export interface Message {
	id: string;
	sender: "user" | "ai";
	text: string;
}

export interface Chat {
	id: number;
	title: string;
	messages: Message[];
	createdAt: number;
	collectionId: number;
}

export function useChats(defaultCollectionId: number) {
	const [chats, setChats] = useState<Chat[]>([]);
	const [activeChatId, setActiveChatId] = useState<number>(0);
	const [isGenerating, setIsGenerating] = useState(false);

	const activeChat = chats.find((c) => c.id === activeChatId) || chats[0];

	// Load chats from DB on mount
	useEffect(() => {
		loadChats();
	}, []);

	// Token streaming handler
	useEffect(() => {
		const offToken = Events.On("chat:token", (e: any) => {
			const targetSessionId = e.data.sessionId;
			setChats((prevChats) =>
				prevChats.map((chat) => {
					if (chat.id === targetSessionId) {
						const lastMsg = chat.messages[chat.messages.length - 1];
						if (lastMsg && lastMsg.sender === "ai") {
							const updatedMessages = [...chat.messages];
							updatedMessages[updatedMessages.length - 1] = {
								...lastMsg,
								text: lastMsg.text + e.data.token,
							};
							return { ...chat, messages: updatedMessages };
						} else {
							return {
								...chat,
								messages: [
									...chat.messages,
									{ id: Math.random().toString(), sender: "ai" as const, text: e.data.token },
								],
							};
						}
					}
					return chat;
				})
			);
		});

		const offDone = Events.On("chat:done", () => {
			setIsGenerating(false);
		});

		return () => {
			offToken();
			offDone();
		};
	}, []);

	const loadChats = async () => {
		try {
			const sessions = (await GetChats()) || [];
			const chatList: Chat[] = [];
			for (const s of sessions) {
				const msgs = (await GetChatMessages(s.id)) || [];
				const messages: Message[] = msgs.map((m) => ({
					id: m.id.toString(),
					sender: m.role === "user" ? "user" : "ai",
					text: m.content,
				}));
				chatList.push({
					id: s.id,
					title: s.title || "New Chat",
					messages,
					createdAt: s.createdAt,
					collectionId: s.collectionId,
				});
			}
			setChats(chatList);
			if (chatList.length > 0) {
				setActiveChatId(chatList[0].id);
			}
		} catch (err) {
			console.error("Failed to load chats:", err);
			// Fall back to a default chat
			handleNewChat();
		}
	};

	const handleNewChat = useCallback(async () => {
		// If there's already an empty chat, switch to it
		const emptyChat = chats.find((c) => c.messages.length === 0);
		if (emptyChat) {
			setActiveChatId(emptyChat.id);
			return;
		}

		try {
			const id = await CreateChat("New Chat", defaultCollectionId);
			const newChat: Chat = {
				id,
				title: "New Chat",
				messages: [],
				createdAt: Date.now(),
				collectionId: defaultCollectionId,
			};
			setChats((prev) => [newChat, ...prev]);
			setActiveChatId(id);
		} catch (err) {
			console.error("Failed to create chat:", err);
		}
	}, [chats, defaultCollectionId]);

	const handleSendMessage = useCallback(
		async (inputMessage: string) => {
			if (!inputMessage.trim() || isGenerating) return;

			// Ensure there's an active chat
			let targetId = activeChatId;
			if (!targetId) {
				try {
					targetId = await CreateChat("New Chat", defaultCollectionId);
					const newChat: Chat = {
						id: targetId,
						title: "New Chat",
						messages: [],
						createdAt: Date.now(),
						collectionId: defaultCollectionId,
					};
					setChats((prev) => [newChat, ...prev]);
					setActiveChatId(targetId);
				} catch (err) {
					console.error("Failed to create chat:", err);
					return;
				}
			}

			const userPrompt = inputMessage;
			setIsGenerating(true);

			// Rename the chat if it's still "New Chat"
			let updatedTitle = chats.find((c) => c.id === targetId)?.title || "New Chat";
			if (updatedTitle === "New Chat") {
				updatedTitle =
					userPrompt.length > 25 ? userPrompt.substring(0, 25) + "..." : userPrompt;
			}

			// Insert user message
			const userMsg: Message = { id: Math.random().toString(), sender: "user", text: userPrompt };
			setChats((prevChats) =>
				prevChats.map((c) => {
					if (c.id === targetId) {
						return { ...c, title: updatedTitle, messages: [...c.messages, userMsg] };
					}
					return c;
				})
			);

			// Trigger Wails Chat completion
			try {
				await SendMessage(targetId, defaultCollectionId, userPrompt);
			} catch (err) {
				console.error(err);
				setIsGenerating(false);
			}
		},
		[activeChatId, chats, defaultCollectionId, isGenerating]
	);

	const handleDeleteChat = useCallback(async (chatId: number) => {
		try {
			await DeleteChat(chatId);
			setChats((prev) => prev.filter((c) => c.id !== chatId));
			if (activeChatId === chatId) {
				setActiveChatId((prev) => {
					const remaining = chats.filter((c) => c.id !== chatId);
					return remaining.length > 0 ? remaining[0].id : 0;
				});
			}
		} catch (err) {
			console.error("Failed to delete chat:", err);
		}
	}, [activeChatId, chats]);

	return {
		chats,
		activeChatId,
		activeChat,
		isGenerating,
		setActiveChatId,
		handleNewChat,
		handleSendMessage,
		handleDeleteChat,
	};
}
