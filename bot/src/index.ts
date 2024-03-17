import dotenv from "dotenv";
import { Probot } from "probot";
import { createClient } from "redis";

import { Worker } from "./worker";

dotenv.config();

const REDIS_URL = process.env.REDIS_URL ?? "redis://localhost:6379";

export default (app: Probot) => {
  app.on("pull_request.opened", async (context) => {
    const issueComment = context.issue({
      body:
        `Beep, boop 🤖  Hi, I'm instruct-lab-bot and I'm going to help you` +
        ` with your pull request. Thanks for you contribution! 🎉\n` +
        `In order to proceed please reply with the following comment:\n` +
        `\`@instruct-lab-bot generate\`\n` +
        `This will trigger the generation of some test data for your` +
        ` contribution. Once the data is generated, I will let you know` +
        ` and you can proceed with the review.`,
    });
    await context.octokit.issues.createComment(issueComment);
  });
  app.on("issue_comment.created", async (context) => {
    const client = await createClient({
      url: REDIS_URL,
    })
      .on("error", (err) => context.log.error("Redis Client Error", err))
      .connect();
    const issueComment = context.payload.comment.body;
    if (issueComment === "@instruct-lab-bot generate") {
      if (context.payload.issue.pull_request == null) {
        const issueComment = context.issue({
          body: `Beep, boop 🤖  Sorry, I can only generate test data for pull requests.`,
        });
        await context.octokit.issues.createComment(issueComment);
        return;
      }
      const issueComment = context.issue({
        body:
          `Beep, boop 🤖  Generating test data for your pull request.\n\n` +
          `This may take a few seconds...`,
      });
      await context.octokit.issues.createComment(issueComment);
      const jobNumber = await client.incr("jobs");
      const prNumber = context.payload.issue.number;
      await client.set(`jobs:${jobNumber}:pr_number`, prNumber);
      if (context.payload.installation?.id !== null) {
        await client.set(
          `jobs:${jobNumber}:installation_id`,
          `${context.payload.installation?.id}`,
        );
      }
      await client.lPush("generate", jobNumber.toString());
    } else if (issueComment.startsWith("@instruct-lab-bot")) {
      const issueComment = context.issue({
        body: `Beep, boop 🤖  Sorry, I don't understand that command`,
      });
      await context.octokit.issues.createComment(issueComment);
    }
    // Don't process the command if it's not for the bot
  });
  (async () => {
    const worker = new Worker(app);
    await worker.poll();
  })();
};
