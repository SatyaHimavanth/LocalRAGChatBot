// frontend/src/components/Sidebar.tsx
import React from "react";
import { Chat } from "../hooks/useChats";

const PlusIcon = () => (
	<svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
		<line x1="12" y1="5" x2="12" y2="19" />
		<line x1="5" y1="12" x2="19" y2="12" />
	</svg>
);

const SearchIcon = () => (
	<svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
		<circle cx="11" cy="11" r="8" />
		<line x1="21" y1="21" x2="16.65" y2="16.65" />
	</svg>
);

const LibraryIcon = () => (
	<svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
		<rect x="3" y="3" width="7" height="7" />
		<rect x="14" y="3" width="7" height="7" />
		<rect x="3" y="14" width="7" height="7" />
		<rect x="14" y="14" width="7" height="7" />
	</svg>
);

interface SidebarProps {
	chats: Chat[];
	activeChatId: number;
	activeTab: "chat" | "search" | "collections";
	onSelectChat: (id: number) => void;
	onNewChat: () => void;
	onTabChange: (tab: "chat" | "search" | "collections") => void;
}

export function Sidebar({
	chats,
	activeChatId,
	activeTab,
	onSelectChat,
	onNewChat,
	onTabChange,
}: SidebarProps) {
	// Group chats by date
	const now = new Date();
	const oneDay = 24 * 60 * 60 * 1000;

	const today = chats.filter(
		(c) => now.getTime() - new Date(c.createdAt).getTime() < oneDay
	);
	const yesterday = chats.filter(
		(c) =>
			now.getTime() - new Date(c.createdAt).getTime() >= oneDay &&
			now.getTime() - new Date(c.createdAt).getTime() < 2 * oneDay
	);
	const older = chats.filter(
		(c) => now.getTime() - new Date(c.createdAt).getTime() >= 2 * oneDay
	);

	const renderChatGroup = (title: string, groupChats: Chat[]) => {
		if (groupChats.length === 0) return null;
		return (
			<div style={{ marginBottom: "16px" }}>
				<div
					style={{
						fontSize: "11px",
						color: "rgba(255,255,255,0.35)",
						textTransform: "uppercase",
						letterSpacing: "0.5px",
						marginBottom: "8px",
						paddingLeft: "12px",
					}}
				>
					{title}
				</div>
				{groupChats.map((chat) => (
					<div
						key={chat.id}
						onClick={() => onSelectChat(chat.id)}
						style={{
							padding: "6px 12px",
							margin: "2px 8px",
							borderRadius: "8px",
							cursor: "pointer",
							fontSize: "13px",
							color: "rgba(255,255,255,0.7)",
							background:
								chat.id === activeChatId ? "rgba(255,255,255,0.08)" : "transparent",
							transition: "background 0.2s",
							whiteSpace: "nowrap",
							overflow: "hidden",
							textOverflow: "ellipsis",
						}}
						onMouseEnter={(e) => {
							if (chat.id !== activeChatId)
								(e.target as HTMLElement).style.background = "rgba(255,255,255,0.04)";
						}}
						onMouseLeave={(e) => {
							if (chat.id !== activeChatId)
								(e.target as HTMLElement).style.background = "transparent";
						}}
					>
						{chat.title}
					</div>
				))}
			</div>
		);
	};

	return (
		<div
			style={{
				width: "240px",
				background: "rgba(255,255,255,0.03)",
				borderRight: "1px solid rgba(255,255,255,0.06)",
				display: "flex",
				flexDirection: "column",
				height: "100%",
				overflow: "hidden",
			}}
		>
			{/* App title */}
			<div
				style={{
					fontSize: "14px",
					fontWeight: 600,
					padding: "16px 14px 12px",
					color: "rgba(255,255,255,0.85)",
					borderBottom: "1px solid rgba(255,255,255,0.06)",
				}}
			>
				LocalRAG Chat
			</div>

			{/* Action buttons */}
			<div style={{ padding: "10px 10px 6px", display: "flex", flexDirection: "column", gap: "4px" }}>
				<button
					onClick={onNewChat}
					style={navButtonStyle}
					onMouseEnter={(e) => (e.target as HTMLElement).style.background = "rgba(255,255,255,0.08)"}
					onMouseLeave={(e) => (e.target as HTMLElement).style.background = "transparent"}
				>
					<PlusIcon /> New Chat
				</button>
				<button
					onClick={() => onTabChange("search")}
					style={{
						...navButtonStyle,
						background:
							activeTab === "search" ? "rgba(255,255,255,0.08)" : "transparent",
					}}
					onMouseEnter={(e) => (e.target as HTMLElement).style.background = "rgba(255,255,255,0.08)"}
					onMouseLeave={(e) => {
						if (activeTab !== "search")
							(e.target as HTMLElement).style.background = "transparent";
					}}
				>
					<SearchIcon /> Universal Search
				</button>
				<button
					onClick={() => onTabChange("collections")}
					style={{
						...navButtonStyle,
						background:
							activeTab === "collections" ? "rgba(255,255,255,0.08)" : "transparent",
					}}
					onMouseEnter={(e) => (e.target as HTMLElement).style.background = "rgba(255,255,255,0.08)"}
					onMouseLeave={(e) => {
						if (activeTab !== "collections")
							(e.target as HTMLElement).style.background = "transparent";
					}}
				>
					<LibraryIcon /> Collections
				</button>
			</div>

			{/* Chat history */}
			<div
				style={{
					flex: 1,
					overflowY: "auto",
					paddingTop: "8px",
					scrollbarWidth: "thin",
					scrollbarColor: "rgba(255,255,255,0.1) transparent",
				}}
			>
				<div
					style={{
						fontSize: "11px",
						color: "rgba(255,255,255,0.35)",
						textTransform: "uppercase",
						letterSpacing: "0.5px",
						marginBottom: "8px",
						paddingLeft: "12px",
					}}
				>
					Chats History
				</div>
				{renderChatGroup("Today", today)}
				{renderChatGroup("Yesterday", yesterday)}
				{renderChatGroup("Older", older)}

				{chats.length === 0 && (
					<div
						style={{
							fontSize: "12px",
							color: "rgba(255,255,255,0.3)",
							padding: "20px",
							textAlign: "center",
						}}
					>
						No chats yet. Start a new one!
					</div>
				)}
			</div>
		</div>
	);
}

const navButtonStyle: React.CSSProperties = {
	display: "flex",
	alignItems: "center",
	gap: "8px",
	padding: "8px 12px",
	border: "none",
	borderRadius: "8px",
	cursor: "pointer",
	fontSize: "13px",
	color: "rgba(255,255,255,0.75)",
	background: "transparent",
	width: "100%",
	textAlign: "left",
	transition: "background 0.2s",
};
