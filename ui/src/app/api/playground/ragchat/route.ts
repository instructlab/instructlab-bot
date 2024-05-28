// src/app/api/playground/ragchat/route.ts
'use server';

import { NextRequest, NextResponse } from 'next/server';
import { ChromaClient, TransformersEmbeddingFunction } from 'chromadb';
import fetch from 'node-fetch';
import https from 'https';
import { PassThrough } from 'stream';
import '../../../../../envConfig';

// Function to query ChromaDB and format the documents
async function fetchDocuments(lastMessageContent: string) {
  try {
    console.log('Initializing Chroma client');
    const client = new ChromaClient({
      path: process.env.CHROMADB_URL,
    });

    console.log('Creating embedding function');
    const embedder = new TransformersEmbeddingFunction();

    console.log('Getting or creating collection');
    const collection = await client.getOrCreateCollection({ name: 'default-collection', embeddingFunction: embedder });

    console.log('Querying the collection');
    const results = await collection.query({
      nResults: 4,
      queryTexts: [lastMessageContent],
    });

    // Uncomment to deny chat requests if there are no documents in the vector DB
    // if (!results.documents.length) {
    //   return null;
    // }

    console.log('Query results:', results);
    const result = results.metadatas[0]
      .map((metadata: any, index: number) => {
        return `Source ${index + 1}) Title: ${metadata.title}, Page: ${metadata.page}, Content: ${results.documents[0][index]}\n`;
      })
      .join('');

    console.log(result);

    return result;
  } catch (error) {
    console.error('Error fetching and formatting documents:', error);
    throw error;
  }
}

export async function POST(req: NextRequest) {
  try {
    const { question, systemRole } = await req.json();
    const apiURL = req.nextUrl.searchParams.get('apiURL');
    const modelName = req.nextUrl.searchParams.get('modelName');

    if (!apiURL || !modelName) {
      return new NextResponse('Missing API URL or Model Name', { status: 400 });
    }

    console.log('Fetching relevant documents from ChromaDB');
    const relevantDocuments = await fetchDocuments(question);

    // Uncomment to deny chat requests if there are no documents in the vector DB
    // if (!relevantDocuments) {
    //   return new NextResponse('No documents found on the vector database. Please upload documentation.', { status: 404 });
    // }

    const messages = [
      {
        role: 'system',
        content: systemRole,
      },
      {
        role: 'user',
        content: `Here is the relevant documentation:\n${relevantDocuments}`,
      },
      {
        role: 'user',
        content: `Answer my next question using only the above documentation. You must also follow the below rules when answering: - Do not make up answers that are not provided in the documentation. - If you are unsure and the answer is not explicitly written in the documentation context, say "Sorry, I don't know how to help with that." - Prefer splitting your response into multiple paragraphs. - Output as markdown with citations based on the documentation.`,
      },
      {
        role: 'user',
        content: `Here is my question:\n${question}`,
      },
    ];

    const requestData = {
      model: modelName,
      messages,
      stream: true,
    };

    const agent = new https.Agent({
      rejectUnauthorized: false,
    });

    const chatResponse = await fetch(`${apiURL}/v1/chat/completions`, {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
        accept: 'application/json',
      },
      body: JSON.stringify(requestData),
      agent: apiURL.startsWith('https') ? agent : undefined,
    });

    if (!chatResponse.body) {
      return new NextResponse('Failed to fetch chat response', { status: 500 });
    }

    const passThrough = new PassThrough();

    chatResponse.body.on('data', (chunk) => {
      const chunkString = chunk.toString();
      const lines = chunkString.split('\n').filter((line) => line.trim() !== '');

      for (const line of lines) {
        if (line.startsWith('data:')) {
          const json = line.replace('data: ', '');
          if (json === '[DONE]') {
            passThrough.end();
            return;
          }

          try {
            const parsed = JSON.parse(json);
            const deltaContent = parsed.choices[0].delta?.content;

            if (deltaContent) {
              passThrough.write(deltaContent);
            }
          } catch (err) {
            console.error('Error parsing chunk:', err);
          }
        }
      }
    });

    chatResponse.body.on('end', () => {
      passThrough.end();
    });

    return new NextResponse(passThrough, {
      headers: {
        'Content-Type': 'text/plain',
      },
    });
  } catch (error) {
    console.error('Error processing request:', error);
    return new NextResponse('Error processing request', { status: 500 });
  }
}
