package seeders

import (
	"fmt"

	"goravel/app/facades"
	"goravel/app/models"
)

// DemoDataSeeder seeds demo users (one per role: user, author, admin),
// sample posts, and comments — for live demo of the Blog API.
type DemoDataSeeder struct{}

func NewDemoDataSeeder() *DemoDataSeeder {
	return &DemoDataSeeder{}
}

func (s *DemoDataSeeder) Signature() string { return "demo_data" }

func (s *DemoDataSeeder) Run() error {
	query := facades.Orm().Query()

	// --- Seed users (one per role) ---
	type seedUser struct {
		name     string
		email    string
		password string
		role     string
	}
	seedUsers := []seedUser{
		{"Demo User", "user@example.com", "password", "user"},
		{"Demo Author", "author@example.com", "password", "author"},
		{"Demo Admin", "admin@example.com", "password", "admin"},
	}

	users := make([]models.User, 0, len(seedUsers))
	for _, su := range seedUsers {
		var existing models.User
		_ = query.Where("email = ?", su.email).First(&existing)
		if existing.ID > 0 {
			users = append(users, existing)
			fmt.Printf("  user exists: %s (%s)\n", su.email, su.role)
			continue
		}

		hashed, err := facades.Hash().Make(su.password)
		if err != nil {
			return err
		}
		u := models.User{
			Name:     su.name,
			Email:    su.email,
			Password: hashed,
			Role:     su.role,
		}
		if err := query.Create(&u); err != nil {
			return err
		}
		users = append(users, u)
		fmt.Printf("  user created: %s (%s, id=%d)\n", su.email, su.role, u.ID)
	}

	// Find author & admin for posts.
	var author, admin models.User
	for _, u := range users {
		if u.Role == "author" {
			author = u
		}
		if u.Role == "admin" {
			admin = u
		}
	}

	// --- Seed posts ---
	type seedPost struct {
		userID    uint
		title     string
		content   string
		published bool
	}
	seedPosts := []seedPost{
		{author.ID, "Welcome to the Blog", "This is the first published post. Feel free to comment!", true},
		{author.ID, "Draft: Upcoming Features", "Work-in-progress post about upcoming features. Only visible to author & admin.", false},
		{admin.ID, "Admin Announcement", "Platform rules and guidelines for all users.", true},
	}

	posts := make([]models.Post, 0, len(seedPosts))
	for _, sp := range seedPosts {
		var existing models.Post
		_ = query.Where("title = ?", sp.title).First(&existing)
		if existing.ID > 0 {
			posts = append(posts, existing)
			fmt.Printf("  post exists: %q\n", sp.title)
			continue
		}
		p := models.Post{
			UserID:    sp.userID,
			Title:     sp.title,
			Content:   sp.content,
			Published: sp.published,
		}
		if err := query.Create(&p); err != nil {
			return err
		}
		posts = append(posts, p)
		fmt.Printf("  post created: %q (published=%v, id=%d)\n", sp.title, sp.published, p.ID)
	}

	// --- Seed comments on first published post ---
	if len(posts) > 0 {
		firstPost := posts[0]
		seedComments := []struct {
			userID  uint
			content string
		}{
			{users[0].ID, "Great first post! Looking forward to more."},
			{admin.ID, "Nice work. Keep it up!"},
		}
		for _, sc := range seedComments {
			var existing models.Comment
			_ = query.Where("post_id = ? AND user_id = ? AND content = ?",
				firstPost.ID, sc.userID, sc.content).First(&existing)
			if existing.ID > 0 {
				continue
			}
			c := models.Comment{
				PostID:  firstPost.ID,
				UserID:  sc.userID,
				Content: sc.content,
			}
			if err := query.Create(&c); err != nil {
				return err
			}
			fmt.Printf("  comment created on post %d by user %d\n", firstPost.ID, sc.userID)
		}
	}

	fmt.Println("\nSeed complete. Demo credentials:")
	fmt.Println("  user@example.com   / password  (role: user)")
	fmt.Println("  author@example.com / password  (role: author)")
	fmt.Println("  admin@example.com  / password  (role: admin)")
	return nil
}
