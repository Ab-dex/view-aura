// Example shows common SDK usage patterns.
package main

import (
	"context"
	"fmt"
	"log"

	sdk "github.com/Ab-dex/view-aura/libs/go-sdk"
	"github.com/Ab-dex/view-aura/libs/go-sdk/auth"
	"github.com/Ab-dex/view-aura/libs/go-sdk/movies"
	"github.com/Ab-dex/view-aura/libs/go-sdk/payment"
	"github.com/Ab-dex/view-aura/libs/go-sdk/ratings"
	"github.com/Ab-dex/view-aura/libs/go-sdk/upload"
	"github.com/Ab-dex/view-aura/libs/go-sdk/watchlist"
)

func main() {
	ctx := context.Background()

	// ── 1. Create the client ────────────────────────────────────────────────
	client := sdk.New(
		"https://api.cinemaos.com",
		sdk.WithAPIKey(""), // start unauthenticated
	)

	// ── 2. Register and get tokens ──────────────────────────────────────────
	auth, err := client.Auth.Register(ctx, auth.RegisterParams{
		Email:       "producer@studio.com",
		Password:    "supersecret",
		DisplayName: "Studio Producer",
	})
	must(err)

	// Inject the access token for all subsequent calls.
	client.SetToken(auth.AccessToken)
	fmt.Println("logged in as", auth.User.DisplayName)

	// ── 3. Browse movies ────────────────────────────────────────────────────
	minRating := 4.0
	result, err := client.Movies.List(ctx, movies.MovieListParams{
		Genres:    []string{"Drama"},
		MinRating: &minRating,
		SortBy:    "avg_rating",
		SortDir:   "desc",
		Limit:     5,
	})
	must(err)
	fmt.Printf("found %d movies\n", result.Total)
	for _, m := range result.Movies {
		fmt.Printf("  • %s (%.1f★)\n", m.Title, m.AvgRating)
	}

	// ── 4. Get movie detail ─────────────────────────────────────────────────
	detail, err := client.Movies.Get(ctx, result.Movies[0].ID)
	must(err)
	fmt.Printf("\n%s — %d credits\n", detail.Title, len(detail.Credits))

	// ── 5. Rate the movie ───────────────────────────────────────────────────
	acting := 5.0
	rating, err := client.Ratings.Upsert(ctx, detail.ID, ratings.RatingParams{
		Overall: 4.5,
		Acting:  &acting,
	})
	must(err)
	fmt.Printf("rated: overall=%.1f acting=%.1f\n", rating.Overall, *rating.Acting)

	// ── 6. Add to watchlist ─────────────────────────────────────────────────
	entry, err := client.Watchlist.SetStatus(ctx, detail.ID, watchlist.SetStatusParams{
		Status: "watched",
	})
	must(err)
	fmt.Println("watchlist status:", entry.Status)

	// ── 7. Check quota before uploading ─────────────────────────────────────
	quota, err := client.Quota.GetStatus(ctx)
	must(err)
	fmt.Printf("storage: %.1f%% used (%s tier)\n", quota.Storage.UsedPct, quota.Tier)

	// ── 8. Initiate an upload ───────────────────────────────────────────────
	upload, err := client.Upload.Initiate(ctx, upload.InitiateParams{
		FileName:    "short-film.mp4",
		ContentType: "video/mp4",
		SizeBytes:   500 * 1024 * 1024, // 500 MB
	})
	must(err)
	fmt.Println("presigned URL (PUT file here):", upload.PresignedURL[:40]+"...")

	// After the PUT to upload.PresignedURL completes:
	complete, err := client.Upload.Complete(ctx, upload.UploadID)
	must(err)
	fmt.Println("asset created:", complete.AssetID, "status:", complete.Status)

	// ── 9. Subscribe to Pro ─────────────────────────────────────────────────
	sub, err := client.Payment.Subscribe(ctx, payment.SubscribeParams{
		Plan:          "pro",
		PaymentMethod: "pm_card_visa", // Stripe test payment method
	})
	must(err)
	fmt.Println("subscribed to plan:", sub.Plan, "until:", sub.CurrentPeriodEnd)

	// ── 10. Follow another user and check feed ───────────────────────────────
	must(client.Social.Follow(ctx, "some-user-uuid"))
	feed, err := client.Social.GetFeed(ctx, 10, 0)
	must(err)
	fmt.Printf("feed: %d entries\n", len(feed.Entries))

	// ── 11. Refresh token before expiry ─────────────────────────────────────
	newPair, err := client.Auth.Refresh(ctx, auth.RefreshToken)
	must(err)
	client.SetToken(newPair.AccessToken)
	fmt.Println("token refreshed")
}

func must(err error) {
	if err != nil {
		log.Fatal(err)
	}
}
