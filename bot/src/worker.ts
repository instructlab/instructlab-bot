import { Probot } from "probot";
import { createClient } from "redis";

export class Worker {
  app: Probot;

  constructor(app: Probot) {
    this.app = app;
  }

  public async poll() {
    const REDIS_URL = process.env.REDIS_URL ?? "redis://localhost:6379";
    const client = await createClient({
      url: REDIS_URL,
    })
      .on("error", (err) => this.app.log.error("Redis Client Error", err))
      .connect();
    const foreverToMakeLiterHappy = true;
    while (foreverToMakeLiterHappy) {
      const results = await client.rPop("results");
      if (results !== null) {
        const prNumber = await client.get(`jobs:${results}:pr_number`);
        if (prNumber == null) {
          this.app.log.error("No PR number found for job %s", results);
          continue;
        }
        const s3Url = await client.get(`jobs:${results}:s3_url`);
        if (s3Url == null) {
          this.app.log.error("No S3 URL found for job %s", results);
          continue;
        }
        const installationId = await client.get(
          `jobs:${results}:installation_id`,
        );
        if (installationId == null) {
          this.app.log.error("No installation ID found for job %s", results);
          continue;
        }
        const issueComment = {
          owner: "instruct-lab",
          repo: "taxonomy",
          issue_number: parseInt(prNumber),
          body:
            `Beep, boop 🤖  The test data has been generated!\n\n` +
            `Find your results [here](${s3Url}).\n\n` +
            `*This URL expires in 5 days.*`,
        };

        const octokit = await this.app.auth(parseInt(installationId));

        await octokit.issues.createComment(issueComment);
      }
    }
  }
}
