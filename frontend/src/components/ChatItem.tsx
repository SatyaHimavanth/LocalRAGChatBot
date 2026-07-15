import React, { useState } from "react";
import { Chat, ThemeVars } from "../types";
import { I } from "./Icons";

interface ChatItemProps {
  chat: Chat;
  isActive: boolean;
  T: ThemeVars;
  onSelect: () => void;
  onCtx: (x: number, y: number) => void;
}

export const ChatItem: React.FC<ChatItemProps> = ({ chat, isActive, T, onSelect, onCtx }) => {
  const [h, setH] = useState(false);
  return (
    <div
      onMouseEnter={() => setH(true)}
      onMouseLeave={() => setH(false)}
      onClick={onSelect}
      style={{
        padding: "6px 12px",
        margin: "2px 8px",
        borderRadius: 8,
        cursor: "pointer",
        fontSize: 13,
        color: chat.archived ? T.text3 : T.text2,
        background: isActive ? "rgba(99,102,241,0.1)" : "transparent",
        display: "flex",
        alignItems: "center",
        justifyContent: "space-between",
        transition: "background 0.15s",
      }}
    >
      <div
        style={{
          flex: 1,
          whiteSpace: "nowrap",
          overflow: "hidden",
          textOverflow: "ellipsis",
          fontStyle: chat.archived ? "italic" : "normal",
          color: chat.archived ? T.text3 : T.text,
        }}
      >
        {chat.archived && "📦 "}
        {chat.pinned && "📌"}
        {chat.title}
      </div>
      {h && (
        <button
          onClick={(e) => {
            e.stopPropagation();
            onCtx(e.clientX, e.clientY);
          }}
          style={{ background: "none", border: "none", cursor: "pointer", color: T.text3, padding: "2px", flexShrink: 0 }}
        >
          <I.Dots />
        </button>
      )}
    </div>
  );
};
