// frontend/src/hooks/useCollections.ts
import { useState, useEffect, useCallback } from "react";
import {
	CreateCollection,
	GetCollections,
	IngestFile,
} from "../../bindings/changeme/internal/app/chatservice";

export interface Collection {
	id: number;
	name: string;
	docCount: number;
	embeddingModel?: string;
	embeddingDims?: number;
	vectorBackend?: string;
	createdAt?: number;
	updatedAt?: number;
}

export interface IngestedDocument {
	id: string;
	collectionId: number;
	filename: string;
	wordCount: number;
	timestamp: string;
}

export function useCollections() {
	const [collections, setCollections] = useState<Collection[]>([
		{ id: 1, name: "Knowledge Base", docCount: 0 },
	]);
	const [activeCollectionId, setActiveCollectionId] = useState(1);
	const [ingestedDocs, setIngestedDocs] = useState<IngestedDocument[]>([]);

	// Load collections from DB on mount
	useEffect(() => {
		loadCollections();
	}, []);

	const loadCollections = async () => {
		try {
			const cols = (await GetCollections()) || [];
			if (cols && cols.length > 0) {
				const mapped: Collection[] = cols.map((c: any) => ({
					id: c.id,
					name: c.name,
					docCount: c.docCount || 0,
					embeddingModel: c.embeddingModel ?? c.EmbeddingModel ?? "",
					embeddingDims: c.embeddingDims ?? c.EmbeddingDims ?? 0,
					vectorBackend: c.vectorBackend ?? c.VectorBackend ?? "sqlite-vec",
					createdAt: c.createdAt ?? c.CreatedAt ?? 0,
					updatedAt: c.updatedAt ?? c.UpdatedAt ?? 0,
				}));
				setCollections(mapped);
				setActiveCollectionId(mapped[0].id);
			}
		} catch (err) {
			console.error("Failed to load collections:", err);
		}
	};

	const handleCreateCollection = useCallback(async (name: string) => {
		if (!name.trim()) return;
		try {
			const newId = await CreateCollection(name);
			const newCol: Collection = { id: newId, name, docCount: 0, embeddingModel: "", embeddingDims: 0, vectorBackend: "sqlite-vec" };
			setCollections((prev) => [...prev, newCol]);
			setActiveCollectionId(newId);
			return newId;
		} catch (err) {
			console.error("Failed to create collection:", err);
			alert("Failed to create collection: " + err);
		}
	}, []);

	const handleIngestFile = useCallback(
		async (fileName: string, fileContent: string) => {
			if (!fileContent.trim()) return;

			// Sanitize filename
			let rawName = fileName.trim();
			if (!rawName) {
				rawName = "pasted-text-document";
			}
			let sanitizedName = rawName.replace(/[^a-zA-Z0-9.-]/g, "_");
			if (!sanitizedName.includes(".")) {
				sanitizedName += ".txt";
			}

			try {
				await IngestFile(activeCollectionId, sanitizedName, fileContent);

				// Update collection doc count
				setCollections((prevCols) =>
					prevCols.map((col) =>
						col.id === activeCollectionId
							? { ...col, docCount: col.docCount + 1 }
							: col
					)
				);

				const words = fileContent.split(/\s+/).length;
				const newDoc: IngestedDocument = {
					id: Math.random().toString(),
					collectionId: activeCollectionId,
					filename: sanitizedName,
					wordCount: words,
					timestamp: new Date().toLocaleTimeString([], {
						hour: "2-digit",
						minute: "2-digit",
					}),
				};

				setIngestedDocs((prevDocs) => [newDoc, ...prevDocs]);
				return "Success! File split, embedded, and ingested successfully.";
			} catch (err) {
				console.error("Ingestion error:", err);
				throw err;
			}
		},
		[activeCollectionId]
	);

	return {
		collections,
		activeCollectionId,
		ingestedDocs,
		setActiveCollectionId,
		handleCreateCollection,
		handleIngestFile,
		reloadCollections: loadCollections,
	};
}
