import { NextRequest, NextResponse } from 'next/server';
import { v4 as uuidv4 } from 'uuid';
import path from 'path';
import fs from 'fs/promises';
import '../../../../../../envConfig';

export async function POST(req: NextRequest): Promise<NextResponse> {
  try {
    console.log('Parsing form data');
    const formData = await req.formData();
    const file = formData.get('file');

    if (!file || !(file instanceof Blob)) {
      console.log('Invalid file');
      return NextResponse.json({ message: 'Invalid file' }, { status: 400 });
    }

    console.log('Reading file buffer');
    const arrayBuffer = await file.arrayBuffer();
    const buffer = Buffer.from(arrayBuffer);

    const tempFilePath = path.join('/tmp', `${uuidv4()}.pdf`);
    console.log('Saving file to', tempFilePath);
    await fs.writeFile(tempFilePath, buffer);

    console.log('Importing dependencies');
    const { PDFLoader } = await import('@langchain/community/document_loaders/fs/pdf');
    const { RecursiveCharacterTextSplitter } = await import('langchain/text_splitter');
    const { ChromaClient, TransformersEmbeddingFunction } = await import('chromadb');

    console.log('Loading PDF');
    const loader = new PDFLoader(tempFilePath);
    const originalDocs = await loader.load();

    console.log('Splitting document');
    const splitter = new RecursiveCharacterTextSplitter({
      chunkSize: 500,
      chunkOverlap: 100,
    });

    const docs = await splitter.splitDocuments(originalDocs);

    console.log('Processing documents');
    const { ids, metadatas, documentContents } = processDocuments(docs);

    console.log('Initializing Chroma client');
    const client = new ChromaClient({
      path: process.env.CHROMADB_URL,
    });

    console.log('Getting or creating collection');
    const embedder = new TransformersEmbeddingFunction();
    const collection = await client.getOrCreateCollection({
      name: 'default-collection',
      embeddingFunction: embedder,
    });

    console.log('Adding documents to collection');
    await collection.add({
      ids,
      metadatas,
      documents: documentContents,
    });

    console.log('Cleaning up temporary file');
    await fs.unlink(tempFilePath);

    console.log('File processed and stored successfully');
    return NextResponse.json(
      {
        message: 'Documents processed successfully',
        documentCount: ids.length,
      },
      { status: 200 }
    );
  } catch (error) {
    console.error('Error processing file:', error);
    return NextResponse.json({ message: 'An error occurred while processing the documents' }, { status: 500 });
  }
}

function processDocuments(docs: any) {
  const ids = [];
  const metadatas = [];
  const documentContents = [];

  for (const document of docs) {
    const id = uuidv4();
    ids.push(id);

    const fallbackTitle = path.basename(document.metadata.source);
    const titleFromMetadata = document.metadata.pdf?.info?.Title;

    const title = titleFromMetadata && titleFromMetadata.length > 0 ? titleFromMetadata : fallbackTitle;

    const metadata = {
      title: title,
      page: document.metadata.loc?.pageNumber,
      source: document.metadata.source,
    };
    metadatas.push(metadata);

    documentContents.push(document.pageContent);
  }

  return { ids, metadatas, documentContents };
}
